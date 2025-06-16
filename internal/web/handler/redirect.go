package handler

import (
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
)

func (s *HandleService) RedirectHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()

	route := request.Context().Value(common.RouteKey).(*config.Route)

	response.SetStatus(http.StatusMovedPermanently)
	response.Headers().Set("Location", route.Data["to"].(string))

	return nil
}
