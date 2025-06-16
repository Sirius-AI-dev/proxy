package handler

import (
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/service/collector"
	"net/http"
)

func (s *HandleService) CollectorHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	route := request.Context().Value(common.RouteKey).(*config.Route)

	tasks := make([]common.Task, 0, len(route.Tasks))
	task := common.ServiceTask{
		Service: collector.ServiceName,
		Data: map[string]interface{}{
			collector.KeyCollector: "default",
			collector.KeyData:      request.Context().Value(common.RequestBodyKey).(*map[string]interface{}),
		},
		Options: common.TaskOptions{
			Group: common.TaskOptionKeyGroupNoWait,
		},
	}
	task.SetContext(request.Context())
	tasks = append(tasks, &task)

	err := s.taskService.NewPipeline(tasks, request, response, nil).Run()
	if err != nil {
		response.SetError(err)
	}
	response.SetStatus(http.StatusNoContent)

	return response
}
