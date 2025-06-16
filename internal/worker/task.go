package worker

import (
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/task"
	"time"
)

const (
	TaskWait     = "task_wait"
	TaskIdleWait = "task_idle_wait"
	TaskTick     = "task_tick"
)

type tasker struct {
	name         string
	listenerType string
	interval     time.Duration
	log          *logger.Logger
	tasksCount   int // Count of tasks
	done         chan bool
	status       bool
	taskService  task.Service

	ticker *time.Ticker

	task   common.ServiceTask
	result common.ServiceTask
	error  common.ServiceTask

	locker chan int

	worker Worker
}

func NewTasker(
	name string,
	listenerType string,
	interval time.Duration,
	log *logger.Logger,
	taskService task.Service,
	tasksCount int,
	count int,
	buffer int,

	taskTask common.ServiceTask,
	taskResult common.ServiceTask,
	taskError common.ServiceTask,
) Process {
	if tasksCount <= 0 {
		tasksCount = 1
	}
	newLog := log.WithField("tasker", name)
	t := &tasker{
		name:         name,
		listenerType: listenerType,
		interval:     interval,
		log:          newLog,
		tasksCount:   tasksCount,
		done:         make(chan bool),
		status:       true,
		taskService:  taskService,

		task:   taskTask,
		result: taskResult,
		error:  taskError,

		locker: make(chan int),
	}
	t.worker = NewWorker(
		buffer,
		taskTask,
		newLog,
		taskService,
		t.locker,
	)

	return t
}

func (t *tasker) Type() string {
	return t.listenerType
}

func (t *tasker) Run() error {
	if t.listenerType == TaskWait {
		return t.RunWithWait()
	} else if t.listenerType == TaskTick {
		return t.RunWithTick()
	} else if t.listenerType == TaskIdleWait {
		return t.RunWithIdleWait()
	}
	return nil
}

// RunWithTick ticker
func (t *tasker) RunWithTick() error {
	t.ticker = time.NewTicker(t.interval)
	doneTicker := make(chan bool)

	go func() {
		for {
			select {
			case <-doneTicker:
				return
			case _ = <-t.ticker.C:
				// TODO: check if worker is running

				t.worker.PushTask(t.task)
			}
		}
	}()

	<-t.done           // Wait done signal for listener
	doneTicker <- true // Send done signal to ticker

	return nil
}

// RunWithWait executes a database query with l.interval times to get new tasks for the worker
func (t *tasker) RunWithWait() error {
	for {
		// pushTask pushing task into worker channel
		t.worker.PushTask(t.task)

		<-t.locker

		// Waiting for interval between tasks execute
		time.Sleep(t.interval)
	}
}

// RunWithIdleWait executes a database query without interval, if proxyTasks[] is empty
func (t *tasker) RunWithIdleWait() error {
	i := 0
	for {
		waiter := make(chan int)
		idleTask := t.task
		idleTask.Waiter = &waiter
		idleTask.TaskId = fmt.Sprintf("%d", i)
		i = i + 1

		t.log.Debugf("Push task %s", idleTask.TaskId)

		// pushTask pushing task into worker channel
		go t.worker.PushTask(idleTask)

		// if proxyTasks[] from first response of database is greather than 0
		if <-*idleTask.Waiter == 0 {
			time.Sleep(t.interval)
		}
	}
}

func (t *tasker) Stop() error {
	t.done <- true
	return nil
}

func (t *tasker) Status() bool {
	return t.status
}
