package eventstream

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/internal/task"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	json                   = jsoniter.ConfigCompatibleWithStandardLibrary
	eventStreamConnections = connections{
		connections: make(map[string]*Connection),
	}
)

const (
	ServiceName       = "eventstream"
	ESIDField         = "es_id"
	ServiceTokenField = "service_token"
	RegisterApiCall   = "/ub/es/register"
	UnregisterApiCall = "/ub/es/unregister"
	ESSelectApiCall   = "/ub/es/select"
	reconnectTimeout  = 5 * time.Second
)

type EventStream interface {
	service.Interface
	AddConnection(*Connection) error
	Send(string, string)
	Notificator(database.Notification)
	ExtractEsIdFromRequest(*http.Request) (string, string, error)
	RegisterConnection(*http.Request) (string, error)
	CreateConnection(*http.Request, http.ResponseWriter, string) *Connection
}

// Notification structure from database
type Notification struct {
	ESList  []map[string]interface{} `json:"es_list"`
	Payload map[string]interface{}   `json:"payload"`
}

type connections struct {
	connections map[string]*Connection
	mutex       sync.Mutex
}

func (cm *connections) Add(connection *Connection) {
	if connection.ESID == "" {
		return
	}
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[connection.ESID] = connection
}
func (cm *connections) Remove(ESID string) {
	if _, ok := cm.connections[ESID]; ok {
		cm.mutex.Lock()
		delete(cm.connections, ESID)
		cm.mutex.Unlock()
	}
}

func (cm *connections) Get(ESID string) *Connection {
	return cm.connections[ESID]
}

func (cm *connections) Send(ESID string, payload string) error {
	if connection, ok := cm.connections[ESID]; ok {
		return connection.Send(payload)
	}
	return fmt.Errorf("event stream connection '%s' not found", ESID)
}

func (cm *connections) Count() int {
	return len(cm.connections)
}

type eventStream struct {
	connections *connections
	mutex       sync.Mutex
	cfg         *config.Config
	db          database.DB
	log         *logger.Logger
	taskService task.Service

	selectStatus bool

	proxyTasksNotificator func(proxyTasks []common.Task)
	pushCountMetricFunc   func()
}

type esResult struct {
	code    int
	headers http.Header
	body    []byte
	err     error
	tasks   []common.Task
}

func (e *esResult) MapData() (mapBody map[string]interface{}, err error) {
	err = json.Unmarshal(e.body, &mapBody)
	return
}

func (e *esResult) Body() []byte {
	return e.body
}

func (e *esResult) Apply(m *map[string]interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (e *esResult) Set(m *map[string]interface{}) {
	//TODO implement me
	panic("implement me")
}

func (e *esResult) Code() int {
	return e.code
}

func (e *esResult) Headers() *http.Header {
	return &e.headers
}

func (e *esResult) Error() error {
	return e.err
}

func (e *esResult) Tasks() []common.Task {
	return e.tasks
}

func New(
	cfg *config.Config,
	db database.DB,
	proxyTasksNotificator func(proxyTasks []common.Task),
	log *logger.Logger,
	taskService task.Service,
) EventStream {
	return &eventStream{
		connections:           &eventStreamConnections,
		cfg:                   cfg,
		db:                    db,
		log:                   log,
		taskService:           taskService,
		proxyTasksNotificator: proxyTasksNotificator,
		pushCountMetricFunc: func() {
			taskService.GetService(metrics.ServiceName).Do(&common.ServiceTask{
				Service: "metrics",
				Data: map[string]interface{}{
					"measurer": "gaugeVec",
					"metric":   "es_connection_count",
					"labels": map[string]interface{}{
						"proxy_id": cfg.ProxyId,
					},
					"value": eventStreamConnections.Count(),
				},
				Options: common.TaskOptions{
					Group: "nowait",
				},
			})
		},
	}
}

func (s *eventStream) ExtractEsIdFromRequest(request *http.Request) (string, string, error) {
	// Extract esId from request
	requestBody := request.Context().Value(common.RequestBodyKey).(*map[string]interface{})
	esId, ok := (*requestBody)["esId"].(string)
	if !ok {
		return "", "", fmt.Errorf("field 'esId' is not string")
	}
	esIdParts := strings.Split(esId, "_")
	return esId, esIdParts[0], nil
}

// RegisterConnection Do register query to database
func (s *eventStream) RegisterConnection(request *http.Request) (string, error) {

	response := common.NewResponse()

	fullEsId, esId, err := s.ExtractEsIdFromRequest(request)
	if err != nil {
		return "", err
	}

	s.log.Errorf("Check connection ESID %s in connections pool", esId)
	// If the connection with esId is present in the connection pool,
	// then we do not try to register a new connection in the database
	if connection := s.connections.Get(esId); connection != nil {
		s.log.Errorf("Connection ESID %s in pool", esId)
		return esId, nil
	}

	tasks := make([]common.Task, 0, 1)

	// Task to Register new EventStream connection
	tasks = append(tasks, &common.ServiceTask{
		Service: database.ServiceName,
		Data: map[string]interface{}{
			"proxy_id":               s.cfg.ProxyId,
			common.ApiCallKey:        RegisterApiCall,
			ESIDField:                fullEsId,
			common.RequestContextKey: request.Context().Value(common.RequestContextKey),
		},
		Options: common.TaskOptions{
			ResponseKey: "es",
		},
	})
	s.log.Errorf("Send registration ESID %s to DB", esId)
	// Run pipeline to register connection on database side
	err = s.taskService.NewPipeline(tasks, nil, response, nil).Run()
	if err != nil {
		return "", err
	}

	/*var registerResponseData map[string]interface{}
	if mapData, errResponseUnmarshal := response.MapData(); errResponseUnmarshal == nil {
		registerResponseData = mapData
	} else {
		response.SetError(err)
	}

	// Get IDs from database
	var newEsId string
	if esId_, errEsId := jsonpath.Get("$.httpResponse.body.es_id", registerResponseData); errEsId == nil {
		newEsId = esId_.(string)
	} else {
		s.log.Error(err)
		return nil, err
	}*/
	return esId, nil
}

// unregisterConnection Do register query to database
func (s *eventStream) unregisterConnection(esId string) {
	// Unregister connection
	s.log.Errorf("Send unregister to DB")
	_ = s.taskService.NewPipeline([]common.Task{&common.ServiceTask{
		Service: "db",
		Data: map[string]interface{}{
			"apicall":  UnregisterApiCall,
			"proxy_id": s.cfg.ProxyId,
			"es_id":    esId,
		},
	}}, nil, nil, nil).Run()

	// Send metrics
	s.pushCountMetricFunc()
}

// CreateConnection creates connection instance
func (s *eventStream) CreateConnection(request *http.Request, w http.ResponseWriter, newEsId string) *Connection {
	return &Connection{
		ESID:        newEsId,
		Writer:      w,
		Flusher:     w.(http.Flusher),
		Logger:      s.log.WithField(ESIDField, newEsId),
		MessageChan: make(chan string),
		Done:        request.Context().Done(),
	}
}

func (s *eventStream) AddConnection(connection *Connection) error {
	// On close function
	connection.onCloseConnectionFunc = func() {
		s.log.Errorf("Remove ESID %s from pool and wait 5 sec", connection.ESID)
		// Try to remove connection from connections pool
		s.connections.Remove(connection.ESID)

		// Wait for new connections
		time.Sleep(reconnectTimeout)

		// Unregister connection in database
		if s.connections.Get(connection.ESID) == nil {
			s.log.Errorf("ESID %s is not exists in pool", connection.ESID)
			s.unregisterConnection(connection.ESID)
		} else {
			s.log.Errorf("ESID %s exists in pool", connection.ESID)
		}
	}
	s.log.Errorf("Add ESID %s to pool", connection.ESID)
	s.connections.Add(connection)
	return nil
}

func (s *eventStream) Send(ESID string, payload string) {
	if err := s.connections.Send(ESID, payload); err != nil {
		s.log.Error(err)
	}
}

// Notificator handle pg_notify from database and call api_call to get notifications
func (s *eventStream) Notificator(notification database.Notification) {
	if s.selectStatus {
		return
	}

	s.selectStatus = true // Selecting status for prevent double selecting

	s.log.Errorf("Receive PG_NOTIFY, SELECT /ub/es/select from DB")

	// Do select messages for proxy_id
	result := common.NewResponse()
	_ = s.taskService.NewPipeline([]common.Task{&common.ServiceTask{
		Service: "db",
		Data: map[string]interface{}{
			"apicall":  ESSelectApiCall,
			"proxy_id": s.cfg.ProxyId,
		},
	}}, nil, result, nil).Run()

	resultData, err := result.MapData()
	if err != nil {
		s.log.Errorf("Error unmarshal response: %v", err)
	}

	// Try to extract tasks
	proxyTasks := s.taskService.ExtractTasksFromMapData(&resultData)
	if len(proxyTasks) > 0 {
		s.log.Errorf("Receive messages for ES from DB %d", len(proxyTasks))
		s.proxyTasksNotificator(proxyTasks)
	} else {
		s.log.Errorf("NO received messages for ES from DB")
	}
	s.selectStatus = false // Release selecting status
}

/*
Do send message
Example 1. Multi send

	{
		"payload": "<any payload data. String, number, object or array>",
		"es_list": [
			{
				"es_id": "<es_id>"
			}
		]
	}

Example 2. Send for one receiver

	{
		"es_id": "<es_id>",
		<any other fields in json format>
	}
*/
func (s *eventStream) Do(task common.Task) service.Result {
	result := &esResult{
		code:    http.StatusOK,
		headers: http.Header{},
	}
	mapData, err := task.MapData()
	if err != nil {
		result.code = http.StatusInternalServerError
		result.err = err
		return result
	}

	// Send messages to eventStream list
	if payload, okPayload := mapData["payload"]; okPayload {
		if esList, okEsList := mapData["es_list"]; okEsList {
			if esListArray, ok := esList.([]interface{}); ok {
				payloadData, _ := json.Marshal(payload)
				for _, es := range esListArray {
					switch es.(type) {
					case map[string]interface{}:
						if esId, ok := es.(map[string]interface{})["es_id"]; ok {
							if esIdStr, ok := esId.(string); ok {
								s.Send(esIdStr, string(payloadData))
							}
						}
					}
				}

				//Send metric "es_message_send_count" via tasks regardless of whether the message was sent or not
				if len(esListArray) > 0 {
					result.tasks = append(result.tasks, &common.ServiceTask{
						Service: "metrics",
						Data: map[string]interface{}{
							"measurer": "counterVec",
							"metric":   "es_message_send_count",
							"labels": map[string]interface{}{
								"proxy_id": s.cfg.ProxyId,
							},
							"value": len(esListArray),
						},
						Options: common.TaskOptions{
							Group: "nowait",
						},
					})
				}
			}
		}
	} else if esId, ok := mapData["es_id"]; ok {
		s.Send(esId.(string), string(task.Body()))
	}

	return result
}

// Connection describe event stream connection to client
type Connection struct {
	ESID string
	i    uint64

	Writer  http.ResponseWriter
	Flusher http.Flusher
	Logger  *logger.Logger

	MessageChan chan string
	Done        <-chan struct{}

	onCloseConnectionFunc func()
	reconnectEnabled      bool
}

func (c *Connection) Listen() {
	// close the channel after exit the function
	defer func() {
		c.Logger.Errorf("client connection is closed")
		close(c.MessageChan)
		c.MessageChan = nil
		go c.onCloseConnectionFunc()
		c.Logger.Debug("client connection is closed")
	}()

	for {
		select {
		case message := <-c.MessageChan:
			if err := c.Send(message); err != nil {
				c.Logger.Error(err)
				return
			}
		case <-c.Done:
			return
		}
	}
}

func (c *Connection) Send(message string) error {
	_, err := fmt.Fprintf(c.Writer, "id: %d\ndata: %s\n\n", c.i, message)
	if err == nil {
		c.i++
	}
	c.Flusher.Flush()
	return err
}
