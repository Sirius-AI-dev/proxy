package collector

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/database"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//=============================================================================
// Service "collector"
// Task data
// collector string - collector name
// data object - any data to pull to collector
// =============================================================================

const ServiceName = "collector"

const KeyCollector = "collector"
const KeyData = "data"

type collectorService struct {
	db         database.DB
	collectors map[string]Collector
	log        *logger.Logger
}

type Service interface {
	service.Interface
	RegisterCollector(string, config.Collector)
	Collector(string) Collector
	ApplyConfig(*config.Config) error
}

func New(cfg map[string]config.Collector, db database.DB, log *logger.Logger) Service {
	collectors := &collectorService{
		db:         db,
		log:        log,
		collectors: make(map[string]Collector),
	}

	for name, collectorConfig := range cfg {
		collectors.RegisterCollector(name, collectorConfig)
	}

	return collectors
}

func (c *collectorService) ApplyConfig(cfg *config.Config) error {
	for collectorName, collectorConfig := range cfg.Collectors {
		if _, ok := c.collectors[collectorName]; ok {
			c.log.Infof("Collector '%s' already registered", collectorName)
			continue
		}
		c.RegisterCollector(collectorName, collectorConfig)
	}
	return nil
}

func (c *collectorService) RegisterCollector(name string, cfg config.Collector) {

	collectData := make(map[string]interface{})
	collectData[common.ApiCallKey] = cfg.ApiCall
	collectData[cfg.Key] = make([]interface{}, cfg.BatchSize)

	pool := c.db.Pool(cfg.Pool)

	log := c.log.WithField("collector", name)

	c.collectors[name] = NewCollector(config.Collector{
		BatchSize:  cfg.BatchSize,
		BufferSize: cfg.BufferSize,
		Timeout:    cfg.Timeout,
	}, func(data []interface{}) {
		collectData[cfg.Key] = data

		if jsonData, err := json.Marshal(collectData); err == nil {
			log.Debugf("Push %d items", len(data))
			_, errQuery := pool.Exec(context.Background(), log, jsonData)
			if errQuery != nil {
				log.Error(err)
			}
		}
	}, log)
	log.Infof("Collector '%s' registered", name)
}

func (c *collectorService) Collector(name string) Collector {
	if collectorItem, ok := c.collectors[name]; ok {
		return collectorItem
	}
	c.log.Errorf("Collector '%s' not found", name)
	return nil
}

// result represents struct of service response
type collectorResult struct {
	status  int
	headers http.Header
	data    *map[string]interface{}
	err     error
}

func (c *collectorResult) Code() int {
	return c.status
}

func (c *collectorResult) Headers() *http.Header {
	return &c.headers
}

func (c *collectorResult) Body() []byte {
	return []byte{}
}

func (c *collectorResult) Error() error {
	return c.err
}

func (c *collectorResult) MapData() (mapBody map[string]interface{}, err error) {
	return nil, nil
}

func (c *collectorResult) Apply(data *map[string]interface{}) error {
	return nil
}

func (c *collectorResult) Set(data *map[string]interface{}) {
}

func (r *collectorResult) Tasks() []common.Task {
	return []common.Task{}
}

func (c *collectorService) Do(task common.Task) service.Result {
	serviceResult := &collectorResult{
		status:  http.StatusOK,
		headers: http.Header{},
	}

	data, err := task.MapData()
	if err != nil {
		serviceResult.err = err
		return serviceResult
	}

	var collectorInstance Collector

	if collectorName, ok := data[KeyCollector]; ok {
		collectorInstance = c.Collector(collectorName.(string))
	} else {
		c.log.Errorf("Collector '%s' not found", collectorName.(string))
		serviceResult.err = fmt.Errorf("collector '%s' not found", collectorName.(string))
		return serviceResult
	}

	if collectorData, ok := data[KeyData]; ok {
		collectorInstance.Collect(collectorData)
	} else {
		c.log.Errorf("Collector 'data' not found")
		serviceResult.err = fmt.Errorf("collector 'data' not found")
	}

	return serviceResult
}
