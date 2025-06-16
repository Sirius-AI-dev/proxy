package handler

import (
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
)

const (
	KeyHttpResponse = "httpResponse"
	KeyHttpCode     = "httpCode"
	KeyHeaders      = "headers"
	KeyBody         = "body"
	KeyError        = "error"
	KeyCode         = "code"
	KeyMessage      = "message"
)

const durationKey = "duration"

func (s *HandleService) DBHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	route := request.Context().Value(common.RouteKey).(*config.Route)
	response := common.NewResponse()
	tasks := make([]common.Task, 0, len(route.Tasks))

	if len(route.Tasks) > 0 {
		for _, taskItem := range route.Tasks {
			taskItem.SetContext(request.Context())
			tasks = append(tasks, &taskItem)
		}
	}

	if len(tasks) == 0 {
		task := common.ServiceTask{
			Service: "db",
		}
		task.SetContext(request.Context())
		tasks = append(tasks, &task)
	}

	// Inject requestBody data to first task
	requestBody := request.Context().Value(common.RequestBodyKey).(*map[string]interface{})
	if mapData, err := tasks[0].MapData(); err == nil {
		_ = utils.MapMerge(requestBody, mapData)
	}
	tasks[0].Set(requestBody)

	// Run pipeline
	err := s.taskService.NewPipeline(tasks, request, response, nil).Run()
	if err != nil {
		response.SetError(err)
	}

	if mapData, err := response.MapData(); err == nil {
		// Handle database error code
		if iDbError, ok := mapData[KeyError]; ok {
			if dbError, ok := iDbError.(map[string]interface{}); ok {
				if dbErrorCode_, ok := dbError[KeyCode]; ok {
					log := request.Context().Value(common.LoggerKey).(*logger.Logger)

					var dbErrorMessage string
					if dbErrorMessage, ok = dbError[KeyMessage].(string); !ok {
						dbErrorMessage = "Unknown exception"
					}
					response.SetError(fmt.Errorf(dbErrorMessage))

					switch dbErrorCode_.(type) {
					case float64:
						response.SetStatus(int(dbErrorCode_.(float64)))

					default:
						response.SetStatus(http.StatusInternalServerError)
					}
					log.AddField("status", fmt.Sprintf("%d", response.Status()))
				}
			}

			// Handle http response
		} else if httpResp, ok := mapData[KeyHttpResponse]; ok {
			if httpResponse, ok := httpResp.(map[string]interface{}); ok {

				// Http code
				if httpCode, ok := httpResponse[KeyHttpCode]; ok {
					response.SetStatus(int(httpCode.(float64)))
				}

				// Headers
				if headers, ok := httpResponse[KeyHeaders]; ok {
					if httpHeaders, ok := headers.(map[string]interface{}); ok {
						for headerKey, headerValue := range httpHeaders {
							switch headerValue.(type) {
							case string:
								response.Headers().Add(headerKey, headerValue.(string))
							case []interface{}:
								for _, arrayHeaderValue := range headerValue.([]interface{}) {
									if headerValueString, ok := arrayHeaderValue.(string); ok {
										response.Headers().Add(headerKey, headerValueString)
									}
								}
							default:
								continue
							}
						}
					}
				}

				// Body
				if body, ok := httpResponse[KeyBody]; ok {
					switch body.(type) {
					case map[string]interface{}:
						bodyData := body.(map[string]interface{})
						response.Set(&bodyData)
					}
				}
			}
		}
	}

	return response
}
