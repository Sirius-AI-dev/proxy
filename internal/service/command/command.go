package command

import (
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"net/http"
	"os/exec"
)

const ServiceName = "command"

type commandService struct {
	log *logger.Logger
}

type Service interface {
	service.Interface
}

type commandResult struct {
	code    int
	headers http.Header
	body    []byte
	err     error
}

func (r *commandResult) Code() int {
	return r.code
}

func (r *commandResult) Headers() *http.Header {
	return &r.headers
}

func (r *commandResult) Body() []byte {
	return r.body
}

func (r *commandResult) Error() error {
	return r.err
}

func (r *commandResult) MapData() (mapBody map[string]interface{}, err error) {
	return nil, nil
}

func (r *commandResult) Apply(data *map[string]interface{}) error {
	return nil
}

func (r *commandResult) Set(data *map[string]interface{}) {
}

func (r *commandResult) Tasks() []common.Task {
	return []common.Task{}
}

func New(log *logger.Logger) Service {
	return &commandService{
		log: log.WithField(logger.ServiceKey, ServiceName),
	}
}

func (s *commandService) Do(task common.Task) service.Result {
	serviceResult := &commandResult{
		code:    200,
		headers: http.Header{},
	}

	taskData, err := task.MapData()
	if err != nil {
		serviceResult.err = err
		return serviceResult
	}

	commandArray := make([]string, 0)

	switch taskData["command"].(type) {
	case []interface{}:
		for _, v := range taskData["command"].([]interface{}) {
			commandArray = append(commandArray, v.(string))
		}
	case []string:
		commandArray = taskData["command"].([]string)
	}

	cmdResult := exec.Command(commandArray[0], commandArray[1:]...)
	stdout, cmdError := cmdResult.Output()

	if cmdError != nil {
		serviceResult.err = cmdError
		serviceResult.code = 500
		return serviceResult
	}

	serviceResult.body = stdout
	return serviceResult
}
