package worker

import (
	"context"
	"encoding/json"
	"github.com/rs/xid"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/task"
)

// notifyListener represents struct of the notifications listener mechanism
type notifier struct {
	db          database.DB
	listener    database.NotifyListener
	notificator database.NotifyHandler
	channel     string

	taskService task.Service
	log         *logger.Logger

	notifyTask common.Task

	done   chan bool
	status bool
}

// Notification structure of worker, which represents notification from database
//type Notification struct {
//	Handler      string  `json:"handler,omitempty"`
//	WaitResponse float64 `json:"waitResponse,omitempty"`
//
//	Connection string `json:"connection,omitempty"`
//	SQL        string `json:"sql,omitempty"`
//
//	Params map[string]interface{} `json:"params,omitempty"`
//}

func NewNotifier(
	name string,
	connection database.Connection,
	channel string,
	notificator database.NotifyHandler,
	log *logger.Logger,
	db database.DB,
	taskService task.Service,

	// notifyParams is struct to inject to query, when notifier received notify from database
	notifyTask common.Task,
) Process {

	listener := connection.NewNotifyListener(name, channel)

	return &notifier{
		listener:    listener,
		channel:     channel,
		notificator: notificator,
		log:         log,
		done:        make(chan bool),
		db:          db,
		taskService: taskService,
		status:      true,
		notifyTask:  notifyTask,
	}
}

func (nl *notifier) Type() string {
	return "notify"
}

func (nl *notifier) Stop() error {
	nl.listener.Stop()
	nl.status = false
	return nil
}
func (nl *notifier) Status() bool {
	return nl.status
}

func (nl *notifier) Run() error {
	nl.listener.Listen(nl.notificator)
	return nil
}

// Notificator is standard algorithm of processing notifications from database (pg_notify)
func (w *workOperator) Notificator(notification database.Notification) {
	var unmarshalledTasks []common.ServiceTask

	err := json.Unmarshal([]byte(notification.Payload), &unmarshalledTasks)
	if err != nil {
		// The information received by the notification from the listened channel is not a json array.
		// We consider such a case as a command for a proxy
		if notifyHandler, err := w.GetNotifyHandler(notification.Payload); err == nil {
			notifyHandler(nil)
			return
		}

		w.log.Errorf("Notify unmarshal error: %v", err)
	}
	tasks := make([]common.Task, 0, len(unmarshalledTasks))

	for _, taskItem := range unmarshalledTasks {
		taskInstance := taskItem
		taskInstance.SetLogger(w.log.WithField("notify_id", xid.New().String()))
		tasks = append(tasks, &taskInstance)
	}

	_ = w.taskService.NewPipeline(tasks, nil, nil, nil).Run()
}

func (w *workOperator) QueryNotification(pool database.Pool, body []byte) {
	ctx := context.Background()
	result, err := pool.Query(ctx, w.log, body)
	if err != nil {
		w.log.Error(err)
		return
	}

	resultData, err := result.MapData()
	if err != nil {
		w.log.Errorf("Error unmarshal response: %v", err)
	}

	proxyTasks := w.taskService.ExtractTasksFromMapData(&resultData)
	if len(proxyTasks) > 0 {
		err = w.taskService.NewPipeline(proxyTasks, nil, nil, nil).Run()
		w.log.Errorf("Error processing proxy tasks: %v", err)
	}
}
