package internal

import (
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	configReader "github.com/unibackend/uniproxy/internal/config/reader"
	"github.com/unibackend/uniproxy/internal/grpc"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/collector"
	"github.com/unibackend/uniproxy/internal/service/command"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/service/eventstream"
	"github.com/unibackend/uniproxy/internal/service/file"
	"github.com/unibackend/uniproxy/internal/service/geoip"
	"github.com/unibackend/uniproxy/internal/service/http"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/internal/service/security"
	"github.com/unibackend/uniproxy/internal/service/storage"
	"github.com/unibackend/uniproxy/internal/task"
	"github.com/unibackend/uniproxy/internal/web"
	"github.com/unibackend/uniproxy/internal/web/handler"
	"github.com/unibackend/uniproxy/internal/worker"
	"go.uber.org/dig"
)

type Container struct {
	container  *dig.Container
	config     *config.Config
	initConfig *config.Config
	mode       string
}

func NewContainer(cfg *config.Config, mode string) (c *Container, err error) {

	// clone init config from file
	// for use on update config from database
	// init config always preferred
	initConfig := *cfg

	c = &Container{
		config:     cfg,
		container:  dig.New(),
		mode:       mode,
		initConfig: &initConfig,
	}

	if err := c.build(cfg); err != nil {
		return nil, err
	}
	return
}

func (c *Container) build(cfg *config.Config) error {

	// Register configuration parts
	if err := c.container.Provide(c.GetConfig); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigDatabase); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigLogger); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigStorage); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigServer); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigMetrics); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigWorkerService); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigGRPCServer); err != nil {
		return err
	}
	if err := c.container.Provide(c.GetConfigCollectors); err != nil {
		return err
	}

	// Register basic system entities
	if err := c.container.Provide(func(cfgLog config.Logger) *logger.Logger {
		return logger.New(cfgLog.Level, cfg.Logger.Handler).WithField("proxy_id", cfg.ProxyId)
	}); err != nil {
		return err
	}
	if err := c.container.Provide(metrics.New); err != nil {
		return err
	}

	// Register database providers
	if err := c.container.Provide(storage.New); err != nil {
		return err
	}
	if err := c.container.Provide(func(config *config.Database, logger *logger.Logger, measurer metrics.Measurer) database.DB {
		return database.New(config, c.GetConfig().ProxyId, logger, measurer)
	}); err != nil {
		return err
	}
	if err := c.container.Invoke(func(db database.DB) {
		db.Init()
	}); err != nil {
		return err
	}

	if err := c.container.Provide(collector.New); err != nil {
		return err
	}

	// Service providers
	if err := c.container.Provide(task.New); err != nil {
		return err
	}
	if err := c.container.Provide(http.New); err != nil {
		return err
	}
	if err := c.container.Provide(file.New); err != nil {
		return err
	}
	if err := c.container.Provide(command.New); err != nil {
		return err
	}

	if err := c.container.Provide(func(cfg *config.Config) geoip.Service {
		return geoip.New(cfg.Server.GeoIP.DB)
	}); err != nil {
		return err
	}

	if err := c.container.Provide(worker.New); err != nil {
		return err
	}
	if err := c.container.Provide(func(cfg *config.Config, db database.DB, taskService task.Service, log *logger.Logger) eventstream.EventStream {
		return eventstream.New(cfg, db, func(proxyTasks []common.Task) {
			for _, proxyTask := range proxyTasks {
				taskService.Do(proxyTask)
			}
		}, log, taskService)
	}); err != nil {
		return err
	}

	if err := c.container.Provide(New); err != nil {
		return err
	}
	if err := c.container.Provide(handler.New); err != nil {
		return err
	}
	if err := c.container.Provide(web.New); err != nil {
		return err
	}
	if err := c.container.Provide(grpc.New); err != nil {
		return err
	}

	// Build Task service
	if err := c.container.Invoke(func(
		taskService task.Service,
		databaseService database.DB,
		proxyService *Proxy,
		webServer web.Server,
		workerService worker.Interface,
		eventStream eventstream.EventStream,
		geoIpService geoip.Service,
		metricsService metrics.Measurer,
		collectorService collector.Service,
		httpService http.Service,
		fileService file.Service,
		commandService command.Service,
		log *logger.Logger,
	) {

		taskService.AddService(database.ServiceName, databaseService)
		taskService.AddService("psql", databaseService) // alias for database with SQL query
		taskService.AddService("jdb", databaseService)  // alias for database with SQL query

		taskService.AddService(http.ServiceName, httpService)
		taskService.AddService(web.ServiceName, webServer)
		taskService.AddService(file.ServiceName, fileService)
		taskService.AddService(geoip.ServiceName, geoIpService)
		taskService.AddService(eventstream.ServiceName, eventStream)
		taskService.AddService(collector.ServiceName, collectorService)
		taskService.AddService(worker.ServiceName, workerService)
		taskService.AddService(metrics.ServiceName, metricsService)
		taskService.AddService(command.ServiceName, commandService)
		taskService.AddService(ServiceName, proxyService)

		workerService.SetMode(c.mode)

		// Config reloader handler
		workerService.AddNotifyHandler("config", func(serviceTask *common.ServiceTask) {
			c.container.Invoke(func(log *logger.Logger) {
				if dbCfg, err := configReader.FromDatabase(log, taskService); err == nil {
					if err = configReader.Apply(dbCfg, taskService, log); err != nil {
						return
					}

					// Reload router as reassign Handler in *http.Server
					taskService.GetService("proxy").Do(&common.ServiceTask{
						Service: "proxy",
						Data: map[string]interface{}{
							KeyOperation: OperationReloadRouter,
						},
					})
				}
			})
		})

		workerService.AddNotificator("standard", workerService.Notificator)

		if c.mode == common.DiModeProxy {
			//workerService.AddNotificator("websocket", websocketService.Notificator)
			workerService.AddNotificator("eventstream", eventStream.Notificator)
		}

		// Apply config from file
		if err := configReader.Apply(c.initConfig, taskService, log); err != nil {
			return
		}

		// Read config from database before container is builded
		dbCfg, err := configReader.FromDatabase(log, taskService)
		if err != nil {
			return
		}
		dbCfg.Apply(c.initConfig) // Merge config with preferred init config
		if err = configReader.Apply(dbCfg, taskService, log); err != nil {
			return
		}
	}); err != nil {
		return err
	}

	// Init security service
	if service, ok := cfg.Services[security.ServiceName]; ok && service.Enabled {
		if err := c.container.Invoke(func(
			workerService worker.Interface,
			taskService task.Service,
			securityService security.Service) {
			taskService.AddService(security.ServiceName, securityService)
			workerService.AddNotificator("blacklist", securityService.Notificator)
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetConfig register configs constructors with parsed config
func (c *Container) GetConfig() *config.Config {
	return c.config
}
func (c *Container) GetConfigDatabase() *config.Database {
	return &c.config.Database
}
func (c *Container) GetConfigDatabaseListeners() map[string]config.DatabaseListener {
	return c.config.Database.Listeners
}
func (c *Container) GetConfigLogger() config.Logger {
	return c.config.Logger
}
func (c *Container) GetConfigServer() *config.Server {
	return &c.config.Server
}
func (c *Container) GetConfigMetrics() config.Metrics {
	return c.config.Metrics
}
func (c *Container) GetConfigGRPCServer() config.GRPCServer {
	return c.config.Server.GRPC
}
func (c *Container) GetConfigWorkerService() *config.WorkerService {
	return &c.config.Worker
}
func (c *Container) GetConfigStorage() map[string]config.Storage {
	return c.config.Storage
}
func (c *Container) GetConfigCollectors() map[string]config.Collector {
	return c.config.Collectors
}
