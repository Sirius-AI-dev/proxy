package handler

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/database"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func (s *HandleService) MaintenanceHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()

	route := request.Context().Value(common.RouteKey).(*config.Route)
	switch route.Data["mode"].(string) {
	case "status":
		return s.statusHandler(request, w)
	case "config":
		return s.configHandler(request, w)
	}

	return response
}

func (s *HandleService) configHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()

	cfg := *s.cfg
	cfg.Server.JwtSecret = "*"
	if data, err := json.Marshal(cfg); err == nil && data != nil {
		response.SetBody(data)
	}

	return response
}

func (s *HandleService) statusHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	db := s.taskService.GetService(database.ServiceName).(database.DB)
	log := request.Context().Value(common.LoggerKey).(*logger.Logger)
	status := statusResponse{}
	status.DB = struct {
		Connections map[string]string `json:"connections"`
		Status      interface{}       `json:"status"`
	}{
		Connections: make(map[string]string),
	}

	statusRequest := common.MapData{
		"apicall": common.EndpointStatus,
	}
	result, statusErr := db.DefaultStatement().Query(context.Background(), log, statusRequest.Marshal())
	if statusErr != nil {
		status.DB.Status = fmt.Sprintf("Status query error %s", statusErr.Error())
	}

	if result != nil && result.Rows() != nil && result.Rows().Next() {
		var dbStatusStr []byte
		errScan := result.Rows().Scan(&dbStatusStr)
		if errScan != nil {
			status.DB.Status = fmt.Sprintf("Status scan error %s", errScan.Error())
		} else {
			var rowStatus map[string]interface{}
			errUnmarshal := json.Unmarshal(dbStatusStr, &rowStatus)
			if errUnmarshal != nil {
				status.DB.Status = fmt.Sprintf("Status unmarshal error %s", errUnmarshal.Error())
			} else if s, ok := rowStatus["status"]; ok {
				status.DB.Status = s
			} else {
				status.DB.Status = rowStatus
			}
		}
	}

	for connectionName, _ := range s.cfg.Database.Connections {
		if db.Connection(connectionName).Status() {
			status.DB.Connections[connectionName] = "ok"
		} else {
			status.DB.Connections[connectionName] = "error"
		}
	}

	if data, err := json.Marshal(status); err == nil && data != nil {
		response.SetBody(data)
	}

	return response
}

type statusResponse struct {
	DB struct {
		Connections map[string]string `json:"connections"`
		Status      interface{}       `json:"status"`
	} `json:"db"`
}
