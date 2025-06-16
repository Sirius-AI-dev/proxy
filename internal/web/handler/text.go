package handler

import (
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
)

func (s *HandleService) TextHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	route := request.Context().Value(common.RouteKey).(*config.Route)

	data, ok := route.Data["data"].(string)
	if !ok {
		response.SetError(fmt.Errorf("data is not a string"))
	}

	httpStatus, ok := route.Data["http_status"].(int)
	if !ok {
		httpStatus = 200
	}

	response.SetBody([]byte(data))
	response.SetStatus(httpStatus)

	return response
}
