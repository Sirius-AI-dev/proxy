package handler

import (
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/task"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
)

const (
	HandleKeyDB          = "db"
	HandleKeyJsonDB      = "jdb"
	HandleKeyHTTP        = "http"
	HandleKeyJsonHTTP    = "jhttp"
	HandleKeyUpload      = "upload"
	HandleKeyMetrics     = "metrics"
	HandleKeyPprof       = "pprof"
	HandleKeySwagger     = "swagger"
	HandleKeyRedoc       = "redoc"
	HandleKeyQueue       = "queue"
	HandleKeyCollector   = "collector"
	HandleKeyRedirect    = "redirect"
	HandleKeyWebsocket   = "websocket"
	HandleKeyEventStream = "eventstream"
	HandleKeyMaintenance = "maintenance"
	HandleKeyText        = "text"
)

type HandleService struct {
	taskService task.Service
	handlers    map[string]common.Handler
	cfg         *config.Config
}

func New(cfg *config.Config, task task.Service) *HandleService {

	h := &HandleService{
		taskService: task,
		handlers:    make(map[string]common.Handler),
		cfg:         cfg,
	}

	h.AddHandler(HandleKeyDB, h.DBHandler)
	h.AddHandler(HandleKeyJsonDB, h.DBHandler)

	h.AddHandler(HandleKeyEventStream, h.EventStreamHandler)
	h.AddHandler(HandleKeyRedirect, h.RedirectHandler)
	h.AddHandler(HandleKeyMetrics, h.MetricsHandler)
	h.AddHandler(HandleKeyUpload, h.UploadHandler)
	h.AddHandler(HandleKeyPprof, h.PProfHandler)
	h.AddHandler(HandleKeyCollector, h.CollectorHandler)
	h.AddHandler(HandleKeyMaintenance, h.MaintenanceHandler)
	h.AddHandler(HandleKeyText, h.TextHandler)

	//h.AddHandler(HandleKeySwagger, h.SwaggerHandler)
	//h.AddHandler(HandleKeyRedoc, h.RedocHandler)

	return h
}

func (s *HandleService) AddHandler(name string, fn common.Handler) {
	s.handlers[name] = fn
}
func (s *HandleService) GetHandler(name string) common.Handler {
	if handler, ok := s.handlers[name]; ok {
		return handler
	}
	return nil
}

func (s *HandleService) DefaultHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	route := request.Context().Value(common.RouteKey).(*config.Route)
	tasks := make([]common.Task, 0, len(route.Tasks))

	if len(route.Tasks) > 0 {
		for _, taskItem := range route.Tasks {
			taskInstance := taskItem
			taskInstance.SetContext(request.Context())
			tasks = append(tasks, &taskInstance)
		}

		requestBody := request.Context().Value(common.RequestBodyKey).(*map[string]interface{})
		if mapData, err := tasks[0].MapData(); err == nil && mapData != nil {
			_ = utils.MapMerge(requestBody, mapData)
		}
		tasks[0].Set(requestBody)
	}

	err := s.taskService.NewPipeline(tasks, request, response, nil).Run()
	if err != nil {
		response.SetError(err)
	}

	return response
}

func (s *HandleService) NotFoundHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	response.SetStatus(http.StatusNotFound)
	response.SetError(fmt.Errorf("endpoint %s not found", request.RequestURI))

	return response
}
