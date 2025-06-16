package handler

import (
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/eventstream"
	"net/http"
)

func (s *HandleService) EventStreamHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	eventStreamService := s.taskService.GetService(eventstream.ServiceName).(eventstream.EventStream)

	response := common.NewResponse()

	log := request.Context().Value(common.LoggerKey).(*logger.Logger)

	// Register or reconnect
	var esId string
	var err error
	if esId, err = eventStreamService.RegisterConnection(request); err != nil {
		response.SetStatus(http.StatusInternalServerError)
		response.SetError(err)
		log.Error(err)
		return response
	}

	eventStreamConnection := eventStreamService.CreateConnection(request, w, esId)
	if eventStreamService.AddConnection(eventStreamConnection) != nil {
		response.SetStatus(http.StatusInternalServerError)
		response.SetError(err)
		log.Error(err)
		return response
	}

	// Prepare the headers
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	if err := eventStreamConnection.Send("{\"client\":\"ok\"}"); err != nil {
		response.SetStatus(http.StatusInternalServerError)
		response.SetError(err)
		log.Error(err)
		return response
	}

	// Trap the request under loop forever
	eventStreamConnection.Listen()

	return response

	/*
		// Create event stream connection
		eventStreamConnection := &eventstream.Connection{
			ESID:        newEsId,
			Writer:      w,
			Flusher:     w.(http.Flusher),
			Logger:      log.WithField(eventstream.ESIDField, newEsId),
			MessageChan: make(chan string),
			Done:        request.Context().Done(),
		}

		// Register event stream connection in service
		if errAddConn := s.taskService.GetService(eventstream.ServiceName).(eventstream.EventStream).AddConnection(eventStreamConnection); errAddConn != nil {
			registerResponse.SetStatus(http.StatusInternalServerError)
			registerResponse.SetError(errAddConn)
			log.Error(err)
			return registerResponse
		}

		//s.taskService.GetService(metrics.ServiceName).Do(&common.ServiceTask{
		//	Service: "metrics",
		//	Data: map[string]interface{}{
		//		"measurer": "gaugeVec",
		//		"metric":   "es_connection_count",
		//		"labels": map[string]interface{}{
		//			"proxy_id": s.cfg.ProxyId,
		//		},
		//		"value": s.taskService.GetService(eventstream.ServiceName).(eventstream.EventStream).GetConnectionsCount(),
		//	},
		//})

	*/
}
