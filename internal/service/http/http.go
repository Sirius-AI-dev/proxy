package http

import (
	"bytes"
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/utils"
	"io"
	"net/http"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	KeyMethod  = "method"
	KeyURL     = "url"
	KeyHeaders = "headers"
	KeyBody    = "body"
	KeyTimeout = "timeout"

	ClientTimeout = 600
)
const ServiceName = "http"

type httpService struct {
	log    *logger.Logger
	client *http.Client
}

type Service interface {
	service.Interface
	request(ctx context.Context, method string, url string, headers http.Header, body []byte) (*http.Response, error)
}

// result represents struct of service response
type httpResult struct {
	status       int
	headers      http.Header
	body         []byte
	data         map[string]interface{}
	err          error
	httpResponse *http.Response
	toClient     bool
}

func (r *httpResult) Code() int {
	return r.status
}

func (r *httpResult) Headers() *http.Header {
	return &r.headers
}

func (r *httpResult) Body() []byte {
	return r.body
}

func (r *httpResult) Error() error {
	return r.err
}

func (r *httpResult) MapData() (mapBody map[string]interface{}, err error) {

	if strings.Contains(r.headers.Get("Content-Type"), "json") {
		err = json.Unmarshal(r.body, &mapBody)
		if !r.toClient {
			mapBody = map[string]interface{}{
				"httpCode": r.status,
				"body":     mapBody, // repack mapBody
				"headers":  r.headers,
			}
		}
		//r.data = mapBody
	} else {
		mapBody = map[string]interface{}{
			"httpCode": r.status,
			"body":     r.body,
			"headers":  r.headers,
		}
	}

	return //nil, fmt.Errorf("format is not json")
}

func (r *httpResult) Apply(data *map[string]interface{}) error {
	return utils.MapMerge(&r.data, data)
}

func (r *httpResult) Set(data *map[string]interface{}) {
	r.data = *data
}

func (r *httpResult) Tasks() []common.Task {
	return []common.Task{}
}

func New(log *logger.Logger) Service {
	return &httpService{
		log: log.WithField(logger.ServiceKey, ServiceName),
		client: &http.Client{
			Timeout: time.Duration(ClientTimeout) * time.Second,
		},
	}
}

func (s *httpService) request(ctx context.Context, method string, url string, headers http.Header, body []byte) (*http.Response, error) {
	request, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	request.Header = headers
	response, err := s.client.Do(request)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	return response, nil
}

func (s *httpService) Do(task common.Task) service.Result {
	serviceResult := &httpResult{
		toClient: task.GetOptions().ToClient,
	}

	taskData, err := task.MapData()
	data := taskData
	if err != nil {
		serviceResult.err = err
		return serviceResult
	}

	method := http.MethodGet
	if method_, ok := data[KeyMethod]; ok {
		method = method_.(string)
	}

	url := ""
	if url_, ok := data[KeyURL]; ok {
		url = url_.(string)
	}

	headers := http.Header{}
	if header, ok := data[KeyHeaders]; ok {
		if headerMap, ok := header.(map[string]interface{}); ok {
			for key, value := range headerMap {
				headers.Add(key, value.(string))
			}
		}
	}

	var requestBody, body []byte
	if bodyData, ok := data[KeyBody]; ok {
		switch bodyData.(type) {
		case string:
			requestBody = []byte(bodyData.(string))
		case map[string]interface{}:
			requestBody, err = json.Marshal(bodyData)
			if err != nil {
				task.GetLogger().Error(err)
			}
		}
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout, ok := data[KeyTimeout]; ok {
		ctx, cancel = context.WithTimeout(ctx, utils.ParseDuration(timeout.(string), ClientTimeout*time.Second))
		defer cancel()
	}

	task.GetLogger().Debugf(fmt.Sprintf("HTTP REQUEST %s TO >>> %s", method, data[KeyURL]))
	var httpResponse *http.Response
	httpResponse, err = s.request(ctx, method, url, headers, requestBody)
	if err != nil {
		task.GetLogger().Error(err)
		serviceResult.err = err
		return serviceResult
	}
	defer httpResponse.Body.Close()

	serviceResult.httpResponse = httpResponse
	serviceResult.status = httpResponse.StatusCode
	serviceResult.headers = httpResponse.Header

	body, err = io.ReadAll(httpResponse.Body)
	if err != nil {
		task.GetLogger().Error(err)
		serviceResult.err = err
		return serviceResult
	}

	task.GetLogger().Debugf(fmt.Sprintf("HTTP RESPONSE <<< %s", body))

	if strings.Contains(httpResponse.Header.Get("Content-Type"), "application/json") {
		var responseBody map[string]interface{}
		err = json.Unmarshal(body, &responseBody)
		if err == nil {
			serviceResult.data = responseBody
		}
	}

	serviceResult.body = body
	return serviceResult
}
