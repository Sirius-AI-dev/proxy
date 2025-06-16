package config

import (
	"github.com/unibackend/uniproxy/internal/common"
	"time"
)

// Config of proxy/worker service
type Config struct {
	// Logger config for one instance of github.com/sirupsen/logrus package
	Logger Logger `yaml:"logger,omitempty" json:"logger,omitempty" mapstructure:"logger,omitempty"`

	// Database config for all database connections
	Database Database `yaml:"database" json:"database" mapstructure:"database"`

	// Handlers config for
	Handlers   map[string]Handler   `yaml:"handlers,omitempty" json:"handlers,omitempty" mapstructure:"handlers,omitempty"`
	Services   map[string]Service   `yaml:"service,omitempty" json:"service,omitempty" mapstructure:"service,omitempty"`
	Server     Server               `yaml:"server" json:"server" mapstructure:"server"` // TODO: rename to webserver
	Metrics    Metrics              `yaml:"metrics,omitempty" json:"metrics,omitempty" mapstructure:"metrics,omitempty"`
	Storage    map[string]Storage   `yaml:"storage,omitempty" json:"storage,omitempty" mapstructure:"storage,omitempty"`
	Collectors map[string]Collector `yaml:"collectors,omitempty" json:"collectors,omitempty" mapstructure:"collectors,omitempty"`
	Worker     WorkerService        `yaml:"worker,omitempty" json:"worker,omitempty" mapstructure:"worker,omitempty"`
	ProxyId    string               `yaml:"proxyId" json:"proxyId" mapstructure:"proxyId"`
}

type Collector struct {
	BatchSize  int    `yaml:"batchSize" json:"batchSize" mapstructure:"batchSize"`
	BufferSize int    `yaml:"bufferSize" json:"bufferSize" mapstructure:"bufferSize"`
	Timeout    string `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	Pool       string `yaml:"pool" json:"pool" mapstructure:"pool"`
	ApiCall    string `yaml:"apicall" json:"apicall" mapstructure:"apicall"`
	Key        string `yaml:"key" json:"key" mapstructure:"key"`
}

type Logger struct {
	Level     string `yaml:"level" json:"level" mapstructure:"level"`
	Handler   string `yaml:"handler" json:"handler" mapstructure:"handler"`
	Collector string `yaml:"collector" json:"collector" mapstructure:"collector"`
}

type Server struct {
	HTTP         HTTPServer        `yaml:"http" json:"http" mapstructure:"http"`
	GRPC         GRPCServer        `yaml:"grpc" json:"grpc" mapstructure:"grpc"`
	Scheme       string            `yaml:"scheme" json:"scheme" mapstructure:"scheme"`
	DefaultRoute string            `yaml:"defaultRoute" json:"defaultRoute" mapstructure:"defaultRoute"`
	Routes       map[string]*Route `yaml:"routes" json:"routes" mapstructure:"routes"`
	Timeout      string            `yaml:"timeout" json:"timeout" mapstructure:"timeout" default:"30s"`
	Domains      []string          `yaml:"domains" json:"domains" mapstructure:"domains"`
	Cors         ServerCors        `yaml:"cors" json:"cors" mapstructure:"cors"`
	JwtSecret    string            `yaml:"jwt-secret" json:"jwt-secret" mapstructure:"jwt-secret"`
	ApiVersion   string            `yaml:"apiVersion" json:"apiVersion" mapstructure:"apiVersion"`
	Middleware   Middleware        `yaml:"middleware" json:"middleware" mapstructure:"middleware"`
	GeoIP        struct {
		DB string `yaml:"db,omitempty" json:"db,omitempty" mapstructure:"db,omitempty"`
	} `yaml:"geoip,omitempty" json:"geoip,omitempty" mapstructure:"geoip,omitempty"`
	ExcludeRequestKeys []string `yaml:"excludeRequestKeys" json:"excludeRequestKeys" mapstructure:"excludeRequestKeys"`
}

type HTTPServer struct {
	Host    string `yaml:"host" json:"host" mapstructure:"host"`
	Port    int    `yaml:"port" json:"port" mapstructure:"port"`
	Timeout string `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}
type GRPCServer struct {
	Host    string `yaml:"host" json:"host" mapstructure:"host"`
	Port    int    `yaml:"port" json:"port" mapstructure:"port"`
	Timeout string `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}

type ServerCors struct {
	Domains       []string `yaml:"domains" json:"domains" mapstructure:"domains"`
	Headers       []string `yaml:"headers" json:"headers" mapstructure:"headers"`
	HeadersString string
}

type Middleware struct {
	Required []map[string]interface{} `yaml:"required" json:"required" mapstructure:"required"`
}

type Metrics struct {
	Runtime struct {
		Duration string `yaml:"duration" json:"duration" mapstructure:"duration"`
		CPU      bool   `yaml:"cpu" json:"cpu" mapstructure:"cpu"`
		Mem      bool   `yaml:"mem" json:"mem" mapstructure:"mem"`
		GC       bool   `yaml:"gc" json:"gc" mapstructure:"gc"`
	} `yaml:"runtime" json:"runtime" mapstructure:"runtime"`
	List map[string]struct {
		Type   string   `yaml:"type" json:"type" mapstructure:"type"`
		Labels []string `yaml:"labels,omitempty" json:"labels,omitempty" mapstructure:"labels,omitempty"`
	} `yaml:"list" json:"list" mapstructure:"list"`
}

type Storage struct {
	Type       string `yaml:"type" json:"type" mapstructure:"type"`
	Host       string `yaml:"host" json:"host" mapstructure:"host"`
	Password   string `yaml:"password,omitempty" json:"password,omitempty" mapstructure:"password,omitempty"`
	Database   string `yaml:"database,omitempty" json:"database,omitempty" mapstructure:"database,omitempty"`
	Expiration int    `yaml:"expiration,omitempty" json:"expiration,omitempty" mapstructure:"expiration,omitempty"`
}

type Database struct {
	Defaults    DatabaseDefaults            `yaml:"defaults" json:"defaults" mapstructure:"defaults"`
	Connections map[string]Connection       `yaml:"connections" json:"connections" mapstructure:"connections"`
	Statements  map[string]Statement        `yaml:"statements" json:"statements" mapstructure:"statements"`
	ApiCallMap  map[string]string           `yaml:"apiCallMap" json:"apiCallMap" mapstructure:"apiCallMap"`
	Pools       map[string]ConnectionsPool  `yaml:"pools" json:"pools" mapstructure:"pools"`
	Listeners   map[string]DatabaseListener `yaml:"listeners" json:"listeners" mapstructure:"listeners"`
	QueryLog    bool                        `yaml:"queryLog,omitempty" json:"queryLog,omitempty" mapstructure:"queryLog,omitempty"`
}

type Connection struct {
	Driver      string        `yaml:"driver" json:"driver" mapstructure:"driver"`
	Type        string        `yaml:"type" json:"type" mapstructure:"type"`
	ReadOnly    bool          `yaml:"readOnly" json:"readOnly" mapstructure:"readOnly"`
	DSN         string        `yaml:"dsn" json:"dsn" mapstructure:"dsn"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	MaxOpenConn int32         `yaml:"maxOpenConn,omitempty" json:"maxOpenConn,omitempty" mapstructure:"maxOpenConn,omitempty"`
	MaxIdleConn int32         `yaml:"maxIdleConn,omitempty" json:"maxIdleConn,omitempty" mapstructure:"maxIdleConn,omitempty"`
	MaxTTLConn  string        `yaml:"maxTtlConn,omitempty" json:"maxTtlConn,omitempty" mapstructure:"maxTtlConn,omitempty"`
}

type Statement struct {
	Timeout    time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	Connection string        `yaml:"connection" json:"connection" mapstructure:"connection"`
	Statement  string        `yaml:"statement" json:"statement" mapstructure:"statement"`
}

type DatabaseDefaults struct {
	Connection string        `yaml:"connection" json:"connection" mapstructure:"connection"`
	Statement  string        `yaml:"statement" json:"statement" mapstructure:"statement"`
	Pool       string        `yaml:"pool" json:"pool" mapstructure:"pool"`
	Timeout    time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}

type ConnectionsPool struct {
	AttemptsFail      int           `yaml:"failattempts" json:"failattempts" mapstructure:"failattempts"`
	AttemptsPeriod    string        `yaml:"failattemptsperiod" json:"failattemptsperiod" mapstructure:"failattemptsperiod"`
	LockPeriod        string        `yaml:"lockperiod" json:"lockperiod" mapstructure:"lockperiod"`
	Statements        []string      `yaml:"statements" json:"statements" mapstructure:"statements"`
	DefaultConnection string        `yaml:"defaultConnection" json:"defaultConnection" mapstructure:"defaultConnection"`
	Timeout           time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}

type DatabaseListener struct {
	Connection string `yaml:"connection" json:"connection" mapstructure:"connection"`
	Channel    string `yaml:"channel" json:"channel" mapstructure:"channel"`
}

type Handler struct {
	Handler string            `yaml:"handler" json:"handler" mapstructure:"handler"`
	Scheme  string            `yaml:"scheme" json:"scheme" mapstructure:"scheme"`
	Host    string            `yaml:"host,omitempty" json:"host,omitempty" mapstructure:"host,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" mapstructure:"headers,omitempty"`
	Timeout time.Duration     `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}

type Service struct {
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
}

type Route struct {
	Handler     string   `yaml:"handler" json:"handler" mapstructure:"handler"`
	Endpoints   []string `yaml:"endpoints" json:"endpoints" mapstructure:"endpoints"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty" mapstructure:"description,omitempty"`
	Methods     []string `yaml:"methods,omitempty" json:"methods,omitempty" mapstructure:"methods,omitempty"`
	Domains     []string `yaml:"domains,omitempty" json:"domains,omitempty" mapstructure:"domains,omitempty"`
	Internal    bool     `yaml:"internal,omitempty" json:"internal,omitempty" mapstructure:"internal,omitempty"`

	// Timeout for execute request to caller service
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout,omitempty"`

	Data map[string]interface{} `yaml:"data,omitempty" json:"data,omitempty" mapstructure:"data,omitempty"`

	Options struct {
		SkipClaimsValidation bool `yaml:"skipClaimsValidation,omitempty" json:"skipClaimsValidation,omitempty"  mapstructure:"skipClaimsValidation,omitempty"`
	} `yaml:"options,omitempty" json:"options,omitempty" mapstructure:"options,omitempty"`
	Middlewares []map[string]interface{} `yaml:"middlewares,omitempty" json:"middlewares,omitempty" mapstructure:"middlewares,omitempty"`
	Tasks       []common.ServiceTask     `yaml:"tasks,omitempty" json:"tasks,omitempty" mapstructure:"tasks,omitempty"`
}

type WorkerService struct {
	// Workers
	Processes map[string]Process `yaml:"processes,omitempty" json:"processes,omitempty"`
}

type Process struct {
	// Type of workerListener
	// "task_(interval|ticker|waiter)" interval query to database and then precessed on worker
	// 		"interval" sleep between executes query
	// 		"ticker" tick every Interval time to execute query
	//      "waiter" sleep between executes query if proxyTasks[] is empty
	// "notify" is listener pg_notify notifications from Postgres
	Type string `yaml:"type" json:"type"`

	// Disabled process (default: false)
	Disabled bool `yaml:"disabled" json:"disabled"`

	// Mode of workerListener
	// "proxy" run on proxy (api-gateway) instance
	// "worker" run on worker instance
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

	// Interval query to database
	// "1s" - one second
	// "5h" - five hours
	Interval string `yaml:"interval" json:"interval"`

	// TaskCount of task processes
	TaskCount int `yaml:"taskCount,omitempty" json:"taskCount,omitempty"`

	// Count of workers (goroutines) to listen of channel
	Count int `yaml:"count" json:"count"`

	// Buffer of worker channel
	Buffer int `yaml:"buffer" json:"buffer"`

	// Prepared Statement to execute
	Statement string `yaml:"statement,omitempty" json:"statement,omitempty"`

	// Connection to listen of pg_notify from Postgres
	Connection string `yaml:"connection,omitempty" json:"connection,omitempty"`

	// Channel of pg_notify notification to listen
	Channel string `yaml:"channel,omitempty" json:"channel,omitempty"`

	// Notificator function to execute when received notification from database
	Notificator string `yaml:"notificator,omitempty" json:"notificator,omitempty"`

	// TaskParams is parameters to inject in query to database to get tasks
	Task common.ServiceTask `yaml:"task,omitempty" json:"task,omitempty"`

	// ResultParams is parameters to inject in query to database to set results of worker process
	Result common.ServiceTask `yaml:"result,omitempty" json:"result,omitempty"`

	// ErrorParams is parameters to inject in query to database error of processing
	Error common.ServiceTask `yaml:"error,omitempty" json:"error,omitempty"`

	// NotifyParams is parameters to inject
	Notify common.ServiceTask `yaml:"notify,omitempty" json:"notify,omitempty"`
}

func (cfg *Config) Apply(newCfg *Config) {
	// Merge routes of server
	for k, v := range newCfg.Server.Routes {
		cfg.Server.Routes[k] = v
	}

	// Merger worker processes
	for k, v := range newCfg.Worker.Processes {
		cfg.Worker.Processes[k] = v
	}
}
