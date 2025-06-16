package collector

import (
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"time"
)

const defaultPushTimeout = 2

type Collector interface {
	Collect(data interface{})
}

type collector struct {
	// Array to collect data for push
	collection []interface{}

	// Channel for collect data from any goroutine
	collector chan interface{}

	// Size of batch to call push method
	batchSize int

	// Buffer size of channel which collect data from another goroutine
	bufferSize int

	log *logger.Logger

	// Timeout when push called
	timeout time.Duration

	timer      *time.Ticker
	pusherFunc func(data []interface{})
}

func NewCollector(cfg config.Collector, pusherFunc func(data []interface{}), log *logger.Logger) Collector {

	cl := &collector{
		collection: make([]interface{}, 0, cfg.BatchSize),
		collector:  make(chan interface{}, cfg.BufferSize),
		timer:      time.NewTicker(utils.ParseDuration(cfg.Timeout, 30*time.Second)),
		pusherFunc: pusherFunc,
		log:        log,
		batchSize:  cfg.BatchSize,
		bufferSize: cfg.BufferSize,
		timeout:    utils.ParseDuration(cfg.Timeout, 30*time.Second),
	}

	go cl.run()

	return cl
}

func (cl *collector) run() {
	counter := -1
	for {
		select {
		case <-cl.timer.C:
			if counter >= 0 {
				cl.pusherFunc(cl.collection[0 : counter+1])
				counter = -1
				cl.collection = make([]interface{}, 0, cl.batchSize)
			}
		case data := <-cl.collector:
			counter++
			if counter == cl.batchSize {
				go cl.pusherFunc(cl.collection)
				cl.timer = time.NewTicker(cl.timeout)
				counter = 0
				cl.collection = make([]interface{}, 0, cl.batchSize)
			}
			cl.collection = append(cl.collection, data)
		}
	}
}

// usage "go Collect"
func (cl *collector) Collect(data interface{}) {
	cl.collector <- data
}
