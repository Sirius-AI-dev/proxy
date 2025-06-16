package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/task"
	"sync"
	"time"
)

const ServiceName = "worker"

type Process interface {
	Run() error
	Type() string

	// Stop do stop worker
	Stop() error

	// Status worker is run?
	Status() bool
}

type Worker interface {
	// Run worker and listen channel for new task
	Run() error

	// PushTask to channel
	PushTask(task common.ServiceTask)
}

type Interface interface {
	service.Interface
	Run() error

	// AddNotificator adding function which processed Notification type
	AddNotificator(string, database.NotifyHandler)
	GetNotificator(string) database.NotifyHandler
	AddNotifyHandler(name string, fn func(*common.ServiceTask))
	GetNotifyHandler(name string) (func(*common.ServiceTask), error)
	Notificator(database.Notification)

	ApplyConfig(*config.WorkerService) error
	SetMode(string)
}

type workOperator struct {
	db             database.DB
	config         *config.Config
	processes      map[string][]Process
	notificators   map[string]database.NotifyHandler
	notifyHandlers map[string]func(notification *common.ServiceTask)
	log            *logger.Logger
	wg             sync.WaitGroup
	taskService    task.Service
	mode           string
}

func New(
	cfg *config.Config,
	db database.DB,
	taskService task.Service,
	log *logger.Logger,
) Interface {
	return &workOperator{
		config:      cfg,
		db:          db,
		log:         log,
		taskService: taskService,

		processes: make(map[string][]Process),

		notificators:   make(map[string]database.NotifyHandler),
		notifyHandlers: make(map[string]func(notification *common.ServiceTask)),
	}

}

// AddNotificator is a function witch received Notification from database by listening channel
func (w *workOperator) AddNotificator(name string, notificator database.NotifyHandler) {
	w.notificators[name] = notificator
}

func (w *workOperator) GetNotificator(name string) database.NotifyHandler {
	if notificator, ok := w.notificators[name]; ok {
		return notificator
	}
	w.log.Errorf("Notificator '%s' not found, using 'standard' notificator", name)
	return w.notificators["standard"]
}

func (w *workOperator) AddNotifyHandler(name string, fn func(*common.ServiceTask)) {
	w.notifyHandlers[name] = fn
}

func (w *workOperator) GetNotifyHandler(name string) (fn func(*common.ServiceTask), err error) {
	fn, ok := w.notifyHandlers[name]
	if !ok {
		err = errors.New("notify handler " + name + " not found")
	}
	return
}

func (w *workOperator) Run() error {

	if err := w.ApplyConfig(nil); err != nil {
		return err
	}

	// Wait all listeners
	w.wg.Wait()

	return nil
}

func (w *workOperator) SetMode(mode string) {
	w.mode = mode
}

// ApplyConfig apply passed config and run processes
func (w *workOperator) ApplyConfig(cfg *config.WorkerService) error {

	if cfg != nil && cfg.Processes != nil {
		w.config.Worker = *cfg
	}

	for name, processCfg := range w.config.Worker.Processes {
		// Skip if process is disabled
		if processCfg.Disabled {
			continue
		}

		// Skip the process if it is running
		if _, ok := w.processes[name]; ok {
			continue
		}

		// Skip if mode of process is distinct from mode of launched instance (proxy | worker)
		if processCfg.Mode != "" && processCfg.Mode != w.mode {
			continue
		}

		var process Process

		switch processCfg.Type {
		case TaskWait, TaskIdleWait, TaskTick:
			jsonTaskParams, _ := json.Marshal(processCfg.Task)
			w.log.Infof("Starting TASK %s tasker '%s' with params '%s', interval %s", w.mode, name, string(jsonTaskParams), processCfg.Interval)

			intervalDuration, err := time.ParseDuration(processCfg.Interval)
			if err != nil {
				return err
			}

			process = NewTasker(
				name,
				processCfg.Type,
				intervalDuration,
				w.log.WithField("tasker", name),
				w.taskService,
				processCfg.TaskCount,
				processCfg.Count,
				processCfg.Buffer,
				processCfg.Task,
				processCfg.Result,
				processCfg.Error,
			)
		case "notify":
			w.log.Infof("Starting NOTIFY %s listener '%s' on channel '%s' with notificator '%s' and params %s",
				w.mode,
				name,
				processCfg.Channel,
				processCfg.Notificator,
				string(processCfg.Notify.Body()),
			)

			process = NewNotifier(
				name,
				w.db.Connection(processCfg.Connection),
				processCfg.Channel,
				w.notificators[processCfg.Notificator],
				w.log.WithField("notifier", name),
				w.db,
				w.taskService,
				&processCfg.Notify,
			)
			break
		default:
			w.log.Errorf("undefined worker type '%s'", processCfg.Type)
			continue
		}

		w.wg.Add(1)

		// Run process in goroutine
		go func(p Process) {
			defer w.wg.Done()
			err := p.Run()
			if err != nil {
				w.log.Errorf("Worker error: %v", err)
			}
		}(process)

		if _, ok := w.processes[name]; !ok {
			w.processes[name] = make([]Process, 0)
		}
		w.processes[name] = append(w.processes[name], process)
	}

	return nil
}

func (w *workOperator) Do(task common.Task) service.Result {
	return nil
}

// worker struct with open Task channel
// receive Task and call any service
type worker struct {
	tasks       chan common.ServiceTask
	taskService task.Service

	// task for call Do of any service
	task common.ServiceTask

	log *logger.Logger

	// runTime is time of start current task
	runTime time.Time

	locker chan int
}

// NewWorker create worker struct for tasker, with goroutine which listen buffered channel
func NewWorker(buffer int, workerTask common.ServiceTask, log *logger.Logger, taskService task.Service, locker chan int) Worker {
	wrk := &worker{
		log:         log,
		taskService: taskService,
		tasks:       make(chan common.ServiceTask, buffer),
		task:        workerTask,
		runTime:     time.Now(),
		locker:      locker,
	}

	// Run worker in goroutine
	go func(w Worker) {
		err := w.Run()
		if err != nil {
			log.Error(err)
		}
	}(wrk)

	return wrk
}

// PushTask to w.tasks channel
func (w *worker) PushTask(workerTask common.ServiceTask) {
	w.tasks <- workerTask
}

// Run worker to listener of w.tasks channel for new task to processing
func (w *worker) Run() error {
	// Listen tasks from channel task_interval | task_ticker
	for {
		workerTask := <-w.tasks
		log := w.log
		if workerTask.GetTaskId() != "" {
			log.WithField("task_id", workerTask.GetTaskId())
		}
		if workerTask.GetWorkerId() != 0 {
			log.AddField("worker_id", fmt.Sprintf("%d", workerTask.GetWorkerId()))
		}
		log.Debugf("Run task")

		tsCurr := time.Now()
		workerTask.SetTsPrev(w.runTime) // set previous timestamp
		workerTask.SetTsCurr(tsCurr)    // set current timestamp
		w.runTime = tsCurr              // set run time

		go func(t common.ServiceTask, taskLog *logger.Logger) {
			if err := w.RunTask(t); err != nil {
				taskLog.Errorf("Error execute task: %v", err)
			}
			if t.Waiter == nil {
				w.locker <- 1 // unlock interval tasker
			}
		}(workerTask, log)
	}
}

// RunTask run WorkerTask
func (w *worker) RunTask(workerTask common.ServiceTask) (err error) {
	return w.taskService.NewPipeline([]common.Task{&workerTask}, nil, nil, nil).Run()
}
