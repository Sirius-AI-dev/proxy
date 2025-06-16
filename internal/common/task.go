package common

import (
	"context"
	"fmt"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"time"
)

type Task interface {
	WithData
	GetService() string
	GetOptions() *TaskOptions
	GetOption(string) interface{}
	GetTaskId() string
	GetWorkerId() int
	GetContext() context.Context
	SetContext(context.Context)
	SetLogger(*logger.Logger)
	GetLogger() *logger.Logger
	GetWaiter() *chan int
}

// The ServiceTask structure represents start task for service and proxy task from service
type ServiceTask struct {
	ctx  context.Context
	log  *logger.Logger
	body []byte // []byte represent of data

	Service string                 `json:"service" yaml:"service"`
	Data    map[string]interface{} `json:"data" yaml:"data"`
	Options TaskOptions            `json:"options,omitempty" yaml:"options,omitempty"`

	Waiter *chan int

	TaskId     string `json:"taskId"`
	WorkerId   int    `json:"workerId"`
	TSPrevious string `json:"ts_prev"`
	TSCurrent  string `json:"ts_curr"`
}

type TaskOptions struct {
	PauseBefore time.Duration `json:"pauseBefore" yaml:"pauseBefore"`
	PauseAfter  time.Duration `json:"pauseAfter" yaml:"pauseAfter"`

	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	ResponseKey string      `json:"responseTo" yaml:"responseTo"`
	CollectKey  interface{} `json:"collectedTo" yaml:"collectedTo"`

	Tasks string `json:"addTaskMode" yaml:"addTaskMode" default:"end"` // end | current | replace
	Group string `json:"taskGroup" yaml:"taskGroup" default:"self"`    // self | nowait | <groupKey>

	OnError string `json:"onerror" yaml:"onerror" default:"stop"` // stop | continue

	ToClient bool `json:"toClient" yaml:"toClient" default:"false"`

	options map[string]interface{}
}

func (t *ServiceTask) GetContext() context.Context {
	if t.ctx == nil {
		t.ctx = context.Background()
	}
	return t.ctx
}
func (t *ServiceTask) SetContext(ctx context.Context) {
	t.ctx = ctx
}

func (t *ServiceTask) GetLogger() *logger.Logger {
	return t.log
}
func (t *ServiceTask) SetLogger(log *logger.Logger) {
	t.log = log
}

func (t *ServiceTask) GetService() string {
	return t.Service
}

func (t *ServiceTask) GetOptions() *TaskOptions {
	return &t.Options
}

func (t *ServiceTask) GetOption(key string) interface{} {
	return t.Options.options[key]
}

func (t *ServiceTask) GetTaskId() string {
	return t.TaskId
}

func (t *ServiceTask) GetWorkerId() int {
	return t.WorkerId
}

func (t *ServiceTask) MapData() (map[string]interface{}, error) {
	return t.Data, nil
}

func (t *ServiceTask) Body() []byte {
	if len(t.body) == 0 && t.Data != nil {
		body, err := json.Marshal(t.Data)
		if err == nil {
			t.body = body
		}
	}
	return t.body
}

func (t *ServiceTask) Apply(m *map[string]interface{}) error {
	return utils.MapMerge(&t.Data, m)
}

func (t *ServiceTask) Set(m *map[string]interface{}) {
	t.Data = *m
}

func (t *ServiceTask) SetTsPrev(tsPrev time.Time) {
	t.TSPrevious = fmt.Sprintf("%d", tsPrev.UnixMicro())
	(t.Data)["ts_prev"] = t.TSPrevious
}

func (t *ServiceTask) SetTsCurr(tsCurr time.Time) {
	t.TSCurrent = fmt.Sprintf("%d", tsCurr.UnixMicro())
	(t.Data)["ts_curr"] = t.TSCurrent
}

func (t *ServiceTask) GetWaiter() *chan int {
	return t.Waiter
}
