package reader

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/collector"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/task"
	"github.com/unibackend/uniproxy/internal/web"
	"github.com/unibackend/uniproxy/internal/worker"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// PrepareFromEnv reads ENV data into config
func PrepareFromEnv(cfg *config.Config) {
	// Prepare handlers
	for serviceId, service := range cfg.Handlers {
		envKey := fmt.Sprintf("%s_ADDR", strings.Replace(strings.ToUpper(serviceId), "-", "_", -1))
		if envValue := os.Getenv(envKey); envValue != "" {
			cfg.Handlers[serviceId] = config.Handler{
				Handler: service.Handler,
				Scheme:  service.Scheme,
				//Queue:   service.Queue,
				Timeout: service.Timeout,
				Host:    envValue,
			}
		}
	}

	dnDSN := os.Getenv("DB_DSN")
	if dnDSN != "" {
		cfg.Database.Connections["master"] = config.Connection{
			Driver:      "pq",
			Type:        "postgres",
			DSN:         dnDSN,
			Timeout:     time.Duration(30) * time.Second,
			MaxOpenConn: 32,
			MaxIdleConn: 16,
		}
	}

	domain := os.Getenv("DOMAIN")
	if domain != "" {
		cfg.Server.Domains = append(cfg.Server.Domains)
	}

	if host := os.Getenv("HTTP_HOST"); host != "" {
		cfg.Server.HTTP.Host = host
	}
	if port := os.Getenv("HTTP_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err == nil {
			cfg.Server.HTTP.Port = p
		}
	}

	if host := os.Getenv("GRPC_HOST"); host != "" {
		cfg.Server.GRPC.Host = host
	}
	if port := os.Getenv("GRPC_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err == nil {
			cfg.Server.GRPC.Port = p
		}
	}

	proxyId := os.Getenv("ID")
	if proxyId != "" {
		cfg.ProxyId = fmt.Sprintf("proxy-%s", proxyId)
	}
}

//func ParseConfig(configPath string) (cfg Config) {
//	dbDSN := os.Getenv("DB_DSN")
//
//	// Read configuration
//	if configPath == "" {
//		if dbDSN == "" {
//			panic("DB_DSN not specified")
//		}
//
//		configPath = "/config.yml"
//	}
//
//	if err := FromFile(configPath, &cfg); err != nil {
//		panic(err)
//	}
//
//	// Read environments
//	PrepareFromEnv(&cfg)
//
//	return cfg
//}

// FromFile reads config from file path into config
func FromFile(path string) (cfg *config.Config, err error) {
	var content []byte
	ext := filepath.Ext(path)

	content, err = os.ReadFile(path)
	if err != nil {
		return
	}

	switch ext {
	case ".yaml", ".yml":
		// Hack for yaml
		configStruct := config.Config{}
		cfg = &configStruct
		err = yaml.Unmarshal(content, cfg)
		return
	case ".json":
		err = json.Unmarshal(content, cfg)
		return
	default:
		return cfg, fmt.Errorf("unsupported config file extension: %s", ext)
	}
}

// FromDatabase reads proxy config from database and apply received config to "web" and "worker"
func FromDatabase(log *logger.Logger, taskService task.Service) (cfg *config.Config, err error) {
	var result database.QueryResult
	var configData string

	log.Infof("New config applying...")

	query := common.MapData{
		"apicall": common.EndpointConfig,
		"mode":    common.ConfigMode,
	}

	result, err = taskService.GetService(database.ServiceName).(database.DB).DefaultStatement().Query(context.Background(), log, query.Marshal())
	if result != nil {
		defer func() {
			if err := result.Rows().Close(); err != nil {
				log.Error(err)
			}
		}()
	}
	if err != nil {
		log.Errorf("Can't fetch config from database: %s", err.Error())
		return
	}

	if result.Rows().Next() {
		if err = result.Rows().Scan(&configData); err != nil {
			log.Errorf("Can't load config from database: %s", err.Error())
			return
		}
	}

	if err = result.Transaction().Commit(context.Background()); err != nil {
		log.Errorf("Can't load config from database: %s", err.Error())
		return
	}

	if err = json.Unmarshal([]byte(configData), &cfg); err != nil {
		log.Errorf("Can't unmarshal config: %s", err.Error())
		return
	}

	return
}

func Apply(cfg *config.Config, taskService task.Service, log *logger.Logger) (err error) {
	// Apply config to Database
	if err = taskService.GetService(database.ServiceName).(database.DB).ApplyConfig(&cfg.Database); err != nil {
		log.Errorf("Can't apply database config: %s", err.Error())
		return
	}
	log.Infof("Connections %d, statements %d", len(cfg.Database.Connections), len(cfg.Database.Statements))

	// Checking that the port is greater than zero (cfg.Server.Port > 0), because the base may return json with an error,
	// but the parsing will work correctly
	// Apply config to WEB service
	if serverService := taskService.GetService(web.ServiceName); serverService != nil && len(cfg.Server.Routes) > 0 {
		if err = serverService.(web.Server).ApplyConfig(&cfg.Server); err != nil {
			log.Errorf("Can't apply server config: %s", err.Error())
			return
		}
		log.Infof("Routes %d", len(cfg.Server.Routes))
	}

	// Apply config to Worker service
	if err = taskService.GetService(worker.ServiceName).(worker.Interface).ApplyConfig(&cfg.Worker); err != nil {
		log.Errorf("Can't apply server config: %s", err.Error())
		return
	}
	log.Infof("Workers %d", len(cfg.Worker.Processes))
	log.Infof("New config from database applied!")

	// Apply config to Collectors
	if err = taskService.GetService(collector.ServiceName).(collector.Service).ApplyConfig(cfg); err != nil {
		log.Errorf("Can't apply collector config: %s", err.Error())
		return
	}

	return
}
