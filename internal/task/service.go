package task

import (
	"fmt"
	"github.com/PaesslerAG/jsonpath"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/xid"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const ServiceName = "task"

const OnErrorStop = "stop"
const OnErrorContinue = "continue"

type Service interface {
	service.Interface
	ExtractTasks([]interface{}) []common.Task
	ExtractTasksFromMapData(mapData *map[string]interface{}) (tasks []common.Task)
	NewPipeline([]common.Task, *http.Request, common.HandlerResponse, *PipelineCollection) *taskPipeline

	AddService(name string, client service.Interface)
	GetService(name string) service.Interface
}

type taskService struct {
	log      *logger.Logger
	services map[string]service.Interface
	cfg      *config.Config
}

func New(cfg *config.Config, log *logger.Logger) Service {
	return &taskService{
		log:      log.WithField(logger.ServiceKey, ServiceName),
		services: make(map[string]service.Interface),
		cfg:      cfg,
	}
}

type taskPipeline struct {
	taskService *taskService
	tasks       []common.Task
	request     *http.Request
	response    common.HandlerResponse
	log         *logger.Logger

	waitGroup pipelineWaitGroup

	mergeMutex sync.Mutex // Mutex for merge data

	pipelineCollection *PipelineCollection
}

type pipelineWaitGroup struct {
	*sync.WaitGroup
	groupName string
}

type PipelineCollection struct {
	mutex sync.Mutex
	data  map[string]interface{}
	log   *logger.Logger
}

func (c *PipelineCollection) Set(key string, data interface{}) {
	c.mutex.Lock()
	c.data[key] = data
	c.mutex.Unlock()
}
func (c *PipelineCollection) Get(key string) interface{} {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if data, ok := c.data[key]; ok {
		return data
	}
	return nil
}

func (c *PipelineCollection) GetByJsonPath(path string) interface{} {
	if strings.HasPrefix(path, "$.") {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		if data, err := jsonpath.Get(path, c.data); err == nil {
			return data
		} else {
			c.log.Error(err)
			return nil
		}
	}
	return nil
}

func (c *PipelineCollection) Collection() map[string]interface{} {
	return c.data
}

// NewPipeline creates new taskPipeline object for thread of http request
func (t *taskService) NewPipeline(tasks []common.Task, request *http.Request, response common.HandlerResponse, pipelineCollection *PipelineCollection) *taskPipeline {
	log := t.log
	if request != nil {
		if requestId := request.Context().Value(chimw.RequestIDKey); requestId != nil {
			log = t.log.WithField("request_id", requestId.(string))
		}
	}

	if pipelineCollection == nil {
		pipelineCollection = &PipelineCollection{
			data: make(map[string]interface{}),
			log:  log,
		}
	}

	return &taskPipeline{
		taskService: t,
		tasks:       tasks,
		request:     request,
		response:    response,
		log:         log.WithField("pipeline", xid.New().String()),

		waitGroup: pipelineWaitGroup{},

		// Collected data of all task responses where specified key responseTo
		pipelineCollection: pipelineCollection,
	}
}

// AddService keep all clients to external services in one place for quick access from anywhere
func (t *taskService) AddService(name string, service service.Interface) {
	t.services[name] = service
}

// GetService after receiving the client, we need to bring it to the appropriate type
func (t *taskService) GetService(name string) service.Interface {
	if serviceInstance, ok := t.services[name]; ok {
		return serviceInstance
	}
	return nil
}

func (p *taskPipeline) Run() (err error) {
	p.log.Debugf("Run pipeline")

	// Process metrics on defer call
	tasksMetrics := map[string][]time.Duration{}
	defer func(metricsMap *map[string][]time.Duration) {
		if len(*metricsMap) == 0 {
			return
		}

		for serviceName, serviceMetrics := range *metricsMap {
			p.taskService.GetService(metrics.ServiceName).Do(&common.ServiceTask{
				Data: map[string]interface{}{
					"measurer": metrics.TypeCounterVec,
					"metric":   "tasks_executed_count",
					"labels": map[string]interface{}{
						"service":  serviceName,
						"proxy_id": p.taskService.cfg.ProxyId,
					},
					"value": len(serviceMetrics),
				},
			})
		}
	}(&tasksMetrics)

	for k := 0; ; k++ {
		if k >= len(p.tasks) {
			break
		}
		task := p.tasks[k]

		log := p.log.WithField("task_service", task.GetService())
		if taskLog := task.GetLogger(); taskLog != nil {
			log.AddFields(taskLog.GetFields())
		}
		task.SetLogger(log)

		//---------------------------------------------------------
		// PREPARE TASK
		//---------------------------------------------------------
		if task.GetOptions().Tasks == "" {
			task.GetOptions().Tasks = common.TaskOptionKeyTasksEnd
		}
		if task.GetOptions().Group == "" {
			task.GetOptions().Group = common.TaskOptionKeyGroupResult
		}

		serviceInstance := p.taskService.GetService(task.GetService())
		if serviceInstance == nil {
			log.Errorf("service '%s' not found", task.GetService())

			if task.GetOptions().OnError == OnErrorContinue {
				continue
			}
			return fmt.Errorf("service '%s not found", task.GetService())
		}

		if task.GetOptions().PauseBefore >= 1 {
			log.Debugf("Wait time by task before request: %v", task.GetOptions().PauseBefore)
			time.Sleep(task.GetOptions().PauseBefore)
		}

		var requestData map[string]interface{}
		if requestData, err = task.MapData(); err != nil {
			log.Errorf("Error unmarshal request: %v", err)
		}

		if p.waitGroup.groupName != "" && p.waitGroup.groupName != task.GetOptions().Group {
			// Wait for all group goroutines is done
			p.waitGroup.Wait()
			p.waitGroup = pipelineWaitGroup{} // Reset waitGroup
		}

		// Prepare to send collected data after waiting all groups of goroutines
		if task.GetOptions().CollectKey != nil {
			totalMapData := make(map[string]interface{})
			collectedData := make(map[string]interface{})

			switch task.GetOptions().CollectKey.(type) {
			case string: // Collect all saved responses to one key
				for key, result := range p.pipelineCollection.Collection() {
					collectedData[key] = result
				}
				totalMapData[task.GetOptions().CollectKey.(string)] = collectedData

			case []string:
				for _, key := range task.GetOptions().CollectKey.([]string) {
					collectedData[key] = p.pipelineCollection.Get(key)
				}
				totalMapData = collectedData
			case map[string]interface{}:
				for key, path := range task.GetOptions().CollectKey.(map[string]interface{}) {
					switch path.(type) {
					case string:
						if strings.HasPrefix(path.(string), "$.") {
							collectedData[key] = p.pipelineCollection.GetByJsonPath(path.(string))
						}
					}
				}
				totalMapData = collectedData
			}

			// Merge request data with preferred data of task
			p.mergeMutex.Lock()
			for k, v := range requestData {
				totalMapData[k] = v
			}
			p.mergeMutex.Unlock()

			task.Set(&totalMapData)
		}

		//---------------------------------------------------------
		// RUN TASK
		//---------------------------------------------------------
		var result service.Result
		var errResult error
		var proxyTasks []common.Task
		var resultData map[string]interface{}
		if slices.Contains([]string{common.TaskOptionKeyGroupSelf, common.TaskOptionKeyGroupResult, common.TaskOptionKeyGroupNoResult}, task.GetOptions().Group) {

			log.Debugf("Do task...")

			timeStart := time.Now()

			// Run task in current thread
			result = serviceInstance.Do(task)

			if _, ok := tasksMetrics[task.GetService()]; !ok {
				tasksMetrics[task.GetService()] = []time.Duration{}
			}
			tasksMetrics[task.GetService()] = append(tasksMetrics[task.GetService()], time.Since(timeStart))

			// Extract mapped data
			resultData, errResult = result.MapData()
			if errResult != nil {
				log.Debugf("Can't unmarshal response: %v", errResult)
			} else {
				// Extract tasks from body of result of service (generated by response of the Service)
				proxyTasks = append(proxyTasks, p.taskService.ExtractTasksFromMapData(&resultData)...)
			}

			// Extract tasks from service result (generated by Do function of serivce)
			if len(result.Tasks()) > 0 {
				proxyTasks = append(proxyTasks, result.Tasks()...)
			}

			// Update response
			if slices.Contains([]string{common.TaskOptionKeyGroupResult, common.TaskOptionKeyGroupSelf}, task.GetOptions().Group) && p.response != nil {
				p.response.SetStatus(result.Code())

				// Set mapped data as preferred or byte represent
				if resultData != nil {
					p.response.Set(&resultData)
				} else {
					p.response.SetBody(result.Body())
				}

				// Update headers from custom http response
				for key, value := range *result.Headers() {
					for _, val := range value {
						p.response.Headers().Add(key, val)
					}
				}
			}

			if result.Error() != nil {
				if task.GetOptions().OnError == OnErrorContinue {
					continue
				}
				return result.Error()
			}

		} else {
			//---------------------------------------------------------
			// PREPARE TASK IN GOROUTINE
			//---------------------------------------------------------
			// If need to run by grouping responses
			if task.GetOptions().Group != common.TaskOptionKeyGroupNoWait {
				if p.waitGroup.groupName == "" { // If groupName is empty, then no group runned
					p.waitGroup = pipelineWaitGroup{
						WaitGroup: &sync.WaitGroup{},
						groupName: task.GetOptions().Group,
					}
				}
				p.waitGroup.Add(1)
			}

			// Run task on new pipeline in thread of goroutine
			go func(backgroundTask common.Task) {
				if backgroundTask.GetOptions().Group != common.TaskOptionKeyGroupNoWait {
					defer p.waitGroup.Done()
				}

				// Set self group of task (without goroutine)
				backgroundTask.GetOptions().Group = common.TaskOptionKeyGroupResult

				// Run new pipeline with current task as start task for new pipeline
				var pipelineResponse common.HandlerResponse // New response for new pipeline
				errPipeline := p.taskService.NewPipeline([]common.Task{backgroundTask}, nil, pipelineResponse, p.pipelineCollection).Run()
				if errPipeline != nil {
					log.Error(errPipeline)
				}
			}(task)
			continue
		}

		if task.GetWaiter() != nil {
			*task.GetWaiter() <- len(proxyTasks) // send data for task_idle_wait
		}

		//---------------------------------------------------------
		// PROXY TASKS
		//---------------------------------------------------------
		if len(proxyTasks) > 0 {
			if task.GetOptions().Tasks == common.TaskOptionKeyTasksEnd {
				// Insert to end of task list
				// task, task, task, proxyTasks[]...
				//                   ^
				for _, t := range proxyTasks {
					p.tasks = append(p.tasks, t)
				}
			} else if task.GetOptions().Tasks == common.TaskOptionKeyTasksCurrent {
				// Insert into current positions of task list
				// task, task, proxyTasks[]..., task
				//             ^

				p.tasks = append(p.tasks[:k], append(proxyTasks, p.tasks[k+1:]...)...)
			} else if task.GetOptions().Tasks == common.TaskOptionKeyTasksReplace {
				// Replace task list on new proxyTasks[]
				p.tasks = proxyTasks
				k = 0 // Reset iterator
			}
		}

		// Save response to specified key into storage
		if task.GetOptions().ResponseKey != "" {
			p.pipelineCollection.Set(task.GetOptions().ResponseKey, resultData)
		}
	}

	return
}

func (t *taskService) ExtractTasksFromMapData(mapData *map[string]interface{}) (tasks []common.Task) {
	if mapData == nil {
		return []common.Task{}
	}

	if kProxyTasks, ok := (*mapData)[common.TasksKey]; ok { // kProxyTasks - key of proxyTasks
		if tProxyTasks, ok := kProxyTasks.([]interface{}); ok { // tProxyTasks - typed of proxyTasks
			return t.ExtractTasks(tProxyTasks)
		}
	}
	return
}

func (t *taskService) ExtractTasks(tasksList []interface{}) (tasks []common.Task) {
	for _, taskItem := range tasksList {
		taskMap := taskItem.(map[string]interface{})

		options := common.TaskOptions{
			PauseBefore: time.Duration(-1),
			PauseAfter:  time.Duration(-1),
			//Timeout:     1 * time.Hour,
			Tasks: common.TaskOptionKeyTasksEnd,    // default "end"
			Group: common.TaskOptionKeyGroupResult, // default "result"
		}

		if taskItemOptions, ok := taskMap[common.TasksKeyOptions]; ok {
			if taskItemOptionsMap, ok := taskItemOptions.(map[string]interface{}); ok {

				if pauseBefore, ok := taskItemOptionsMap[common.TasksKeyPauseBefore]; ok && len(pauseBefore.(string)) >= 1 {
					if duration, err := time.ParseDuration(pauseBefore.(string)); err == nil {
						options.PauseBefore = duration
					}
				}
				if pauseAfter, ok := taskItemOptionsMap[common.TasksKeyPauseAfter]; ok && len(pauseAfter.(string)) >= 1 {
					if duration, err := time.ParseDuration(pauseAfter.(string)); err == nil {
						options.PauseAfter = duration
					}
				}
				if timeout, ok := taskItemOptionsMap[common.TasksKeyTimeout]; ok && len(timeout.(string)) >= 1 {
					if duration, err := time.ParseDuration(timeout.(string)); err == nil {
						options.Timeout = duration
					}
				}

				if toClient, ok := taskItemOptionsMap[common.ToClientKey]; ok {
					options.ToClient = toClient.(bool)
				}

				if responseKey, ok := taskItemOptionsMap[common.TasksKeyResponseKey]; ok && len(responseKey.(string)) >= 1 {
					options.ResponseKey = responseKey.(string)
				}
				if collectKey, ok := taskItemOptionsMap[common.TasksKeyCollectedKey]; ok {
					options.CollectKey = collectKey
				}
				if groupKey, ok := taskItemOptionsMap[common.TasksKeyGroupKey]; ok && len(groupKey.(string)) >= 1 {
					options.Group = groupKey.(string)
				}
				if tasksKey, ok := taskItemOptionsMap[common.TasksKeyTaskKey]; ok && len(tasksKey.(string)) >= 1 {
					options.Tasks = tasksKey.(string)
				}
			}
		}

		task := common.ServiceTask{
			Service: taskMap[common.TasksKeyService].(string),
			Options: options,
		}

		var taskData map[string]interface{}
		if data, ok := taskMap[common.TasksKeyData]; ok {
			taskData = data.(map[string]interface{})
		} else if request, ok := taskMap[common.TasksKeyRequest]; ok {
			taskData = request.(map[string]interface{})
		}
		task.Data = taskData

		tasks = append(tasks, &task)
	}
	return
}

func (t *taskService) Do(task common.Task) (response service.Result) {
	tasks := []common.Task{task}
	err := t.NewPipeline(tasks, nil, nil, nil).Run()
	if err != nil {
		t.log.Error(err)
	}

	/*switch task.Data.(type) {
	case []types{}:
		var tasks []common.Task
		if tasksList, ok := task.Data.([]types{}); ok {
			tasks = t.ExtractTasks(tasksList)
		}
		t.logger.Debugf("Process tasks: %d", len(tasks))
		if len(tasks) > 0 {
			err = t.ProcessTasks(tasks, &http.Request{
				Logger: t.logger,
			}, &http.PipelineResponse{})
			t.logger.Error(err)
		}
	}*/

	return response
}
