package service

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Interface Every service can be run execute a Task
type Interface interface {
	Do(task common.Task) Result
}

// Result represents result of service
// Every service must implement its own result structure with all Result and types.WithData methods
type Result interface {
	common.WithData
	Code() int
	Headers() *http.Header
	Error() error
	Tasks() []common.Task
}

type CommonResult struct {
	ServiceResult interface{}
	status        int
	headers       http.Header
	body          []byte
	data          map[string]interface{}
	err           error
}

func (r *CommonResult) MapData() (mapBody map[string]interface{}, err error) {
	if r.data != nil {
		return r.data, nil
	}
	err = json.Unmarshal(r.body, &mapBody)
	r.data = mapBody
	return
}

func (r *CommonResult) Body() []byte {
	return r.body
}

func (r *CommonResult) Apply(data *map[string]interface{}) error {
	return utils.MapMerge(&r.data, data)
}

func (r *CommonResult) Set(data *map[string]interface{}) {
	r.data = *data
}
func (r *CommonResult) Tasks() []common.Task {
	return []common.Task{}
}

func (r *CommonResult) Code() int {
	return r.status
}

func (r *CommonResult) Headers() *http.Header {
	return &r.headers
}

func (r *CommonResult) Error() error {
	return r.err
}

func (r *CommonResult) SetError(err error) {
	r.err = err
}

func (r *CommonResult) SetCode(code int) {
	r.status = code
}
