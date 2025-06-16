package database

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/xid"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	StatementKey    = "statement"
	PoolKey         = "pool"
	ServiceName     = "db"
	RollbackKey     = "rollback"
	ProxyOptionsKey = "proxyOptions"

	ModeQueryKey   = "query"
	ModeNoWaitKey  = "nowait"
	ModeExecuteKey = "execute"

	logResultSize = 10000
)

//=============================================================================
// Service "db"
// Task data fields
// apicall string - inject specified apicall to query

// Task options
// statement string - statement for execute query
// pool string - or pool to execute query
// timeout string - timeout for query execution as context timeout (200ms, 1s, 5m, 3h)
// noWait int - 1 = Exec query without waiting for response from database
// =============================================================================

type Query interface {
	Query(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
	Exec(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
}

type db struct {
	config      *config.Database
	log         *logger.Logger
	connections map[string]Connection
	statements  map[string]Statement
	measurer    metrics.Measurer
	pools       map[string]Pool
	listeners   map[string]NotifyListener

	proxyId string
}

type dbResult struct {
	result  QueryResult
	headers *http.Header
	data    map[string]interface{}
	err     error
	code    int
}

func (r *dbResult) MapData() (map[string]interface{}, error) {
	if r.data == nil {
		if r.result == nil {
			r.data = map[string]interface{}{}
		} else {
			if data, err := r.result.MapData(); err == nil {
				r.data = data
			} else {
				return nil, err
			}
		}
	}
	return r.data, nil
}

func (r *dbResult) Body() []byte {
	if r.result == nil {
		return []byte{}
	}
	return r.result.Body()
}

func (r *dbResult) Apply(m *map[string]interface{}) error {
	return utils.MapMerge(&r.data, m)
}

func (r *dbResult) Set(m *map[string]interface{}) {
	r.data = *m
}

func (r *dbResult) Code() int {
	return r.code
}

func (r *dbResult) Headers() *http.Header {
	return r.headers
}

func (r *dbResult) Error() error {
	return r.err
}

func (r *dbResult) Tasks() []common.Task {
	return []common.Task{}
}

type DB interface {
	service.Interface
	RegisterConnection(string, Connection)
	Connection(string) Connection
	RegisterStatement(string, Statement)
	Statement(string) Statement
	DefaultStatement() Statement
	DefaultPool() Pool
	RegisterListener(string, NotifyListener)
	Listener(string) NotifyListener
	Pool(string) Pool
	PoolByApiCall(string) Pool
	Init()
	Status() bool
	ApplyConfig(*config.Database) error
}

func New(config *config.Database, proxyId string, log *logger.Logger, measurer metrics.Measurer) DB {
	return &db{
		config:      config,
		log:         log.WithField(logger.ServiceKey, ServiceName),
		connections: make(map[string]Connection),
		statements:  make(map[string]Statement),
		measurer:    measurer,
		pools:       make(map[string]Pool),
		listeners:   make(map[string]NotifyListener),
		proxyId:     proxyId,
	}
}

func (d *db) RegisterConnection(name string, connection Connection) {
	d.connections[name] = connection
}

func (d *db) Connection(name string) Connection {
	if connection, ok := d.connections[name]; ok {
		return connection
	}
	d.log.Errorf("Connection '%s' not found", name)
	return nil
}

func (d *db) RegisterListener(name string, listener NotifyListener) {
	d.listeners[name] = listener
}

func (d *db) Listener(name string) NotifyListener {
	if listener, ok := d.listeners[name]; ok {
		return listener
	}
	d.log.Errorf("Listener '%s' not found", name)
	os.Exit(1)
	return nil
}

func (d *db) Pool(name string) Pool {
	if pool, ok := d.pools[name]; ok {
		return pool
	}
	d.log.Errorf("Pool '%s' not found", name)
	os.Exit(1)
	return nil
}

func (d *db) Status() bool {
	return d.connections[d.config.Defaults.Connection].Status()
}

func (d *db) Init() {
	// Initiate config from config file for first config query
	d.ApplyConfig(nil)
}

// PoolByApiCall
// priority:
// 1. ApiCall map
// 2. Default pool
func (d *db) PoolByApiCall(apiCall string) Pool {
	var poolName string
	if p, ok := d.config.ApiCallMap[apiCall]; ok {
		poolName = p
	}

	// Get pool instance based on a request
	if poolName == "" {
		poolName = d.config.Defaults.Pool
	}

	return d.Pool(poolName)
}

func (d *db) ApplyConfig(cfg *config.Database) (err error) {
	/*if cfg != nil {
		if d.config == nil {
			d.config = cfg
		} else {
			d.config.Connections = cfg.Connections
			d.config.Statements = cfg.Statements
			d.config.Pools = cfg.Pools
		}
	} // else keep old config, just reinit
	*/
	// Connections
	for connectionName, conn := range d.config.Connections {

		if _, ok := d.connections[connectionName]; ok {
			continue // Skip if connection is established
		}

		dsnUrl, err := url.Parse(conn.DSN)
		if err != nil {
			d.log.Errorf("Invalid db DSN: (%v) %s", err, conn.DSN)
			os.Exit(1)
		}

		d.log.Debugf("Trying to connect to %s (%s%s)", connectionName, dsnUrl.Host, dsnUrl.Path)

		// Apply proxyId to DSN string
		conn.DSN = applyDSNProxyId(conn.DSN, d.proxyId)

		var connection Connection
		switch conn.Driver {
		case "pgxpool":
			connection = NewPgxPoolConnection(conn, d.config, d.log)
		case "pgx":
			connection = NewPgxConnection(conn, d.config, d.log)
		case "pq":
			connection = NewPqConnection(conn, d.config, d.log)
		}

		d.RegisterConnection(connectionName, connection)
	}

	// Statements
	for statementName, stmt := range d.config.Statements {

		if _, ok := d.statements[statementName]; ok {
			continue // Skip if statement is created
		}

		timeout := stmt.Timeout
		if timeout == 0 {
			timeout = d.Connection(stmt.Connection).Timeout()
		}

		d.RegisterStatement(statementName, NewStatement(
			d.Connection(stmt.Connection),
			d.log,
			d.config.Statements[statementName].Statement,
			fmt.Sprintf("%s:%s", stmt.Connection, statementName),
			stmt.Connection,
			d.measurer,
			timeout,
		))
	}

	// Pools
	for poolName, pool := range d.config.Pools {
		if _, ok := d.pools[poolName]; ok {
			continue // Skip if pool is created
		}

		d.pools[poolName] = NewPool(pool, d.log)

		for _, statement := range pool.Statements {
			d.pools[poolName].AddStatement(d.Statement(statement))
		}
	}

	return
}

// Do execute a query on statement or pool
func (d *db) Do(task common.Task) service.Result {
	result := &dbResult{
		code:    http.StatusOK,
		headers: &http.Header{},
	}

	// Logger instance for query
	log := task.GetLogger().WithField("query_id", xid.New().String()) //d.log

	// Get data for query
	data, err := task.MapData()
	if err != nil {
		log.Error(err)
		result.err = err
		return result
	}

	queryMode := ModeQueryKey // Standard mode, when apicall is specified
	var connection string
	var sql []string

	if task.GetService() == "psql" {
		// Check if we need call raw SQL query on connection
		sqlValue, sqlOk := data["sql"]
		connectionValue, connectionOk := data["connection"]
		if sqlOk && connectionOk {
			switch sqlValue.(type) {
			case string: // sql value as single string
				sql = make([]string, 1)
				sql = append(sql, sqlValue.(string))
			case []string: // sql value as array of strings
				sql = make([]string, len(sqlValue.([]string)))
				for _, s := range sqlValue.([]string) {
					sql = append(sql, s)
				}
			}

			connection = connectionValue.(string)
			queryMode = ModeExecuteKey
		} else {
			result.code = http.StatusBadRequest
			result.err = fmt.Errorf("key 'connection' or sql on 'data' not found")
			return result
		}
	}

	// Call ub.api_call without wait the response
	if task.GetOptions().Group == common.TaskOptionKeyGroupNoWait {
		queryMode = ModeNoWaitKey
	}

	var queryResult QueryResult

	switch queryMode {
	case ModeExecuteKey:
		conn := d.Connection(connection)
		if conn != nil {
			tx, errTx := conn.BeginTx(task.GetContext())
			if errTx != nil {
				log.Errorf("EXECUTE TX error: %v", errTx)
			}
			for _, s := range sql {
				log.Debugf("EXECUTE SQL on connection %s >>> %s", connection, sql)
				if _, err := conn.Exec(task.GetContext(), s); err != nil {
					log.Errorf("EXECUTE error: %v", err)
					errTx = tx.Rollback(task.GetContext())
					if errTx != nil {
						log.Errorf("EXECUTE TX error: %v", err)
					}

				}
			}
			errTx = tx.Commit(task.GetContext())
			if errTx != nil {
				log.Errorf("EXECUTE TX error: %v", err)
			}

		}
	case ModeQueryKey, ModeNoWaitKey:
		data["proxy_id"] = d.proxyId

		// apiCall pool mapping
		apiCall := ""
		if ac, ok := data[common.ApiCallKey]; ok { // allocate memory
			apiCall = ac.(string)
		}

		var queryInstance Query
		if statementName := task.GetOption(StatementKey); statementName != nil {
			queryInstance = d.statements[statementName.(string)]
		} else if apiCall != "" {
			queryInstance = d.PoolByApiCall(apiCall)
		} else if poolName := task.GetOption(PoolKey); poolName != nil {
			queryInstance = d.pools[poolName.(string)]
		} else {
			queryInstance = d.DefaultPool()
		}

		// Marshal query data
		var queryData []byte
		queryData, err = json.Marshal(data)
		if err != nil {
			log.Error(err)
			result.err = err
			return result
		}

		if d.config.QueryLog {
			if len(queryData) > logResultSize {
				log.Debugf("DATABASE QUERY >>> %s...........................", queryData[:logResultSize])
			} else {
				log.Debugf("DATABASE QUERY >>> %s", queryData)
			}
		}

		var cancel context.CancelFunc
		ctx := task.GetContext()

		// Get timeout from options
		timeoutDuration := task.GetOptions().Timeout
		if timeoutDuration > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeoutDuration)
			defer cancel()
		} else if timeout := task.GetOption(common.TimeoutKey); timeout != nil {
			if duration, err := time.ParseDuration(timeout.(string)); err == nil {
				ctx, cancel = context.WithTimeout(ctx, duration)
				defer cancel()
			}
		}

		// Execute query
		if queryMode == ModeExecuteKey { // without waiting for response
			_, err = queryInstance.Exec(ctx, log, queryData)
		} else {
			queryResult, err = queryInstance.Query(ctx, log, queryData)
			// Do not forget commit or rollback transaction on result.Transaction()
		}
	}

	result.result = queryResult
	if err != nil {
		result.code = http.StatusInternalServerError
		result.err = err
	} else {
		if d.config.QueryLog && queryResult != nil {
			resultLog := queryResult.Body()
			if len(resultLog) > logResultSize {
				log.Debugf("DATABASE RESULT <<< %s...........................", resultLog[:logResultSize])
			} else {
				log.Debugf("DATABASE RESULT <<< %s", resultLog)
			}
		}
	}

	return result
}

func applyDSNProxyId(dsn string, proxyId string) string {
	r, err := regexp.Compile("#application_name=(.+)&?#")
	if err != nil {
		return dsn
	}

	if r.MatchString(dsn) {
		dsn = r.ReplaceAllString(dsn, fmt.Sprintf("application_name=%s&", proxyId))
	} else {
		dsn = fmt.Sprintf("%s&application_name=%s", dsn, proxyId)
	}

	return dsn
}
