package common

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type UserID int64

const (
	ApiVersionKey   = "apiVersion"
	UniProxyVersion = "0.0.1"

	ServiceNameDatabase    = "db"
	ServiceNameEventStream = "eventstream"
	ServiceNameHTTP        = "http"
	ServiceNameCollector   = "collector"

	EndpointConfig    = "/ub/proxy/config"
	EndpointBlackList = "/ub/blacklist"
	EndpointStatus    = "/ub/status"

	ConfigMode = "proxy_config"

	DiModeProxy  = "proxy"
	DiModeWorker = "worker"

	TasksKey   = "proxyTasks"
	ApiCallKey = "apicall"
	TimeoutKey = "timeout"

	LoggerKey         = "logger"
	ResponseKey       = "handlerResponse"
	RouteKey          = "route"
	RequestBodyKey    = "requestBody"
	RequestContextKey = "request_context"

	TasksKeyPauseBefore  = "pauseBefore"
	TasksKeyPauseAfter   = "pauseAfter"
	TasksKeyTimeout      = "timeout"
	ToClientKey          = "toClient"
	TasksKeyResponseKey  = "responseTo"
	TasksKeyCollectedKey = "collectedTo"
	TasksKeyTaskKey      = "addTaskMode"
	TasksKeyGroupKey     = "taskGroup"

	TasksKeyService = "service"
	TasksKeyRequest = "request"
	TasksKeyData    = "data"
	TasksKeyOptions = "options"

	TaskOptionKeyTasksEnd     = "end"
	TaskOptionKeyTasksCurrent = "current"
	TaskOptionKeyTasksReplace = "replace"

	TaskOptionKeyGroupSelf     = "self"
	TaskOptionKeyGroupResult   = "result"
	TaskOptionKeyGroupNoResult = "noresult"
	TaskOptionKeyGroupNoWait   = "nowait"

	TasksKeyBodyRequest    = "body"
	TasksKeyURL            = "url"
	TasksKeyHostRequest    = "host"
	TasksKeyMethodRequest  = "method"
	TasksKeyHeadersRequest = "headers"
	TasksKeySchemeRequest  = "scheme"
	TasksKeyEndpoint       = "endpoint"
	TasksKeyApiCall        = "apicall"
)

type WithData interface {
	MapData() (map[string]interface{}, error)
	Body() []byte
	Apply(*map[string]interface{}) error
	Set(*map[string]interface{})
}

type WithHttpData interface {
	Status() int
	Headers() *http.Header
	Body() []byte
	Error() error

	SetStatus(int)
	SetHeaders(*http.Header)
	SetBody([]byte)
	SetError(error)
}

type HandlerResponse interface {
	WithHttpData
	WithData
	GetData() interface{}
	RemoveKey(string)
}

type handlerResponse struct {
	status  int
	data    interface{} // alias of "body"
	headers *http.Header
	err     error
}

func NewResponse() HandlerResponse {
	return &handlerResponse{
		status:  http.StatusOK,
		headers: &http.Header{},
	}
}

func (r *handlerResponse) SetBody(body []byte) {
	r.data = body
}
func (r *handlerResponse) SetStatus(status int) {
	r.status = status
}
func (r *handlerResponse) SetHeaders(headers *http.Header) {
	r.headers = headers
}
func (r *handlerResponse) SetError(err error) {
	r.err = err
}
func (r *handlerResponse) GetData() interface{} {
	return r.data
}
func (r *handlerResponse) Status() int {
	return r.status
}
func (r *handlerResponse) Headers() *http.Header {
	return r.headers
}
func (r *handlerResponse) Error() error {
	return r.err
}
func (r *handlerResponse) Body() []byte {
	switch r.data.(type) {
	case []byte:
		return r.data.([]byte)
	case string:
		return []byte(r.data.(string))
	default:
		if r.data != nil {
			if data, err := json.Marshal(r.data); err == nil {
				return data
			}
		}
		return []byte{}
	}
}

func (r *handlerResponse) MapData() (map[string]interface{}, error) {
	switch r.data.(type) {
	case []byte:
		var data map[string]interface{}
		err := json.Unmarshal(r.data.([]byte), &data)
		if err == nil {
			return data, nil
		}
		return nil, err
	case string:
		var data map[string]interface{}
		err := json.Unmarshal([]byte(r.data.(string)), &data)
		if err == nil {
			return data, nil
		}
		return nil, err
	case map[string]interface{}:
		data := r.data.(map[string]interface{})
		return data, nil
	case *map[string]interface{}:
		return *r.data.(*map[string]interface{}), nil
	default:
		return nil, nil
	}
}

func (r *handlerResponse) Apply(m *map[string]interface{}) error {
	switch r.data.(type) {
	case map[string]interface{}:
		data := r.data.(map[string]interface{})
		err := utils.MapMerge(&data, m)
		if err != nil {
			return err
		}
		r.data = data
	case *map[string]interface{}:
		err := utils.MapMerge(r.data.(*map[string]interface{}), m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *handlerResponse) Set(m *map[string]interface{}) {
	r.data = m
}

func (r *handlerResponse) RemoveKey(key string) {
	if mapData, ok := r.data.(map[string]interface{}); ok {
		delete(mapData, key)
	} else if mapData, ok := r.data.(*map[string]interface{}); ok {
		delete(*mapData, key)
	}
}

// Handler is a custom function for extends data
type Handler func(*http.Request, http.ResponseWriter) HandlerResponse

type MapData map[string]interface{}

func (m *MapData) Marshal() []byte {
	data, _ := json.Marshal(m)
	return data
}

func (m *MapData) Unmarshal(b []byte) error {
	return json.Unmarshal(b, m)
}
