package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"strings"
	"time"
)

type pqConnection struct {
	config     config.Connection
	log        *logger.Logger
	connection *sql.DB
	timeout    time.Duration
}

type pqRows struct {
	rows *sql.Rows
}

type pqRow struct {
	row *sql.Row
}

type pgTransaction struct {
	tx *sql.Tx
}

func (p pgTransaction) Commit(ctx context.Context) error {
	return p.tx.Commit()
}

func (p pgTransaction) Rollback(ctx context.Context) error {
	return p.tx.Rollback()
}

type pqResult struct {
	tx         *pgTransaction
	result     sql.Result
	rows       pqRows
	row        pqRow
	statement  Statement
	ctx        context.Context
	log        *logger.Logger
	connection Connection
	data       *map[string]interface{}
	body       []byte
}

func (r *pqResult) Body() []byte {
	r.scan() // read result
	return r.body
}

func (r *pqResult) Apply(m *map[string]interface{}) error {
	return utils.MapMerge(r.data, m)
}

func (r *pqResult) Set(m *map[string]interface{}) {
	r.data = m
}

func (r *pqResult) Transaction() Transaction {
	return r.tx
}

func (r *pqResult) ProxyTasks() []common.Task {
	return make([]common.Task, 0)
}

type pqListener struct {
	name     string
	listener *pq.Listener
	log      *logger.Logger

	done   chan bool
	status bool
}

func NewPqConnection(cfg config.Connection, cfgDB *config.Database, log *logger.Logger) Connection {
	connection, err := sql.Open(cfg.Type, cfg.DSN)
	if err != nil {
		log.Error(fmt.Errorf("failed to connect to database: %v", err))
	}

	if cfg.MaxOpenConn > 0 {
		connection.SetMaxOpenConns(int(cfg.MaxOpenConn))
	}
	if cfg.MaxIdleConn > 0 {
		connection.SetMaxIdleConns(int(cfg.MaxIdleConn))
	}

	if cfg.MaxTTLConn != "" {
		ttlDuration, err := time.ParseDuration(cfg.MaxTTLConn)
		if err != nil {
			panic(err)
		}
		if ttlDuration.Nanoseconds() > 0 {
			connection.SetConnMaxLifetime(ttlDuration)
		}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = cfgDB.Defaults.Timeout
	}

	return &pqConnection{
		config:     cfg,
		log:        log,
		connection: connection,
		timeout:    timeout,
	}
}

func (s *pqConnection) Connection() interface{} {
	return s.connection
}

func (s *pqConnection) Query(ctx context.Context, sqlQuery string, args ...interface{}) (result QueryResult, err error) {
	var tx *sql.Tx
	var pqRowRaw *sql.Row

	sqlQuery = fmt.Sprintf("/*%v\n*/\n%s", strings.ReplaceAll(strings.ReplaceAll(string(args[0].([]byte)), "/*", "/ *"), "*/", "* /"), sqlQuery)

	if s.config.ReadOnly {
		// Replica behaviour (without transaction)
		pqRowRaw = s.connection.QueryRowContext(ctx, sqlQuery, args...) // One row and one column with json
	} else {
		// Master behaviour
		tx, err = s.connection.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		pqRowRaw = tx.QueryRowContext(ctx, sqlQuery, args...) // One row and one column with json
	}

	result = &pqResult{
		ctx: ctx,
		log: s.log,
		tx: &pgTransaction{
			tx: tx,
		},
		row: pqRow{
			row: pqRowRaw,
		},
		connection: s,
	}

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return
}

func (s *pqConnection) Exec(ctx context.Context, sql string, args ...interface{}) (result QueryResult, err error) {
	res, errResult := s.connection.Exec(sql, args...)
	if errResult != nil {
		return nil, errResult
	}

	return &pqResult{
		ctx:    ctx,
		log:    s.log,
		result: res,
	}, nil
}

func (s *pqConnection) Status() bool {
	return s.connection.Stats().OpenConnections > 0
}

func (s *pqConnection) Timeout() time.Duration {
	return s.timeout
}
func (s *pqConnection) Config() *config.Connection {
	return &s.config
}

func (s *pqConnection) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.connection.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &pgTransaction{
		tx: tx,
	}, nil
}

func (r *pqResult) AffectedRows() int64 {
	affectedRows, _ := r.result.RowsAffected()
	return affectedRows
}

func (r *pqResult) LastInsertId() interface{} {
	lastInsertId, _ := r.result.LastInsertId()
	return lastInsertId
}

func (r *pqResult) Rows() Rows {
	if r.rows.rows != nil {
		return &r.rows
	} else if r.row.row != nil {
		return &r.row
	}
	return nil
}

func (r *pqResult) Statement() Statement {
	return r.statement
}

func (r *pqResult) SetStatement(statement Statement) {
	r.statement = statement
}

func (r *pqResult) MapData() (data map[string]interface{}, err error) {
	var response map[string]interface{}
	r.scan() // read result

	if !r.connection.Config().ReadOnly {
		defer func() {
			// Rollback transaction if response have key "rollback" in "proxyOptions" structure
			proxyOptions, ok := response[ProxyOptionsKey]
			if ok {
				if proxyRollback, ok := proxyOptions.(map[string]interface{})[RollbackKey]; ok && proxyRollback == true {
					if err = r.tx.Rollback(r.ctx); err != nil {
						r.log.Error(err)
					}
					return
				}
			}

			// Commit transaction if no error
			if commitErr := r.tx.Commit(r.ctx); commitErr != nil {
				r.log.Error(commitErr)
			}
		}()
	}

	if err = json.Unmarshal(r.body, &response); err != nil {
		return
	}

	return response, nil
}

func (r *pqResult) scan() {
	if len(r.body) == 0 && r.Rows() != nil {
		if r.Rows().Next() {
			var jResponse []byte
			if err := r.Rows().Scan(&jResponse); err != nil {
				r.log.Error(err)
				return // Handle the error
			}

			r.body = jResponse
		}
	}
}

func (r *pqRows) Close() error {
	return r.rows.Close()
}

func (r *pqRows) Err() error {
	return r.rows.Err()
}

func (r *pqRows) Next() bool {
	return r.rows.Next()
}

func (r *pqRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *pqRow) Close() error {
	return nil
}

func (r *pqRow) Err() error {
	return nil
}

func (r *pqRow) Next() bool {
	return true
}

func (r *pqRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

func (s *pqConnection) NewNotifyListener(listenerName string, channel string) NotifyListener {

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			s.log.Errorf(err.Error())
		}
	}

	listener := pq.NewListener(s.config.DSN, 10*time.Second, time.Minute, reportProblem)
	err := listener.Listen(channel)
	if err != nil {
		panic(err)
	}

	return &pqListener{
		name:     listenerName,
		listener: listener,
		log:      s.log,
	}
}

// Listen listening now notifications from database and do call for each Notificator
func (l *pqListener) Listen(notificator NotifyHandler) {
	go func() {
		for {
			select {
			case n := <-l.listener.Notify:
				if n == nil {
					l.log.Debug("EMPTY NOTIFY")
					continue
				}

				l.log.Debugf("NOTIFY on channel '%s' with payload: %s", n.Channel, n.Extra)
				go func() {
					notificator(Notification{
						Channel: n.Channel,
						Payload: n.Extra,
					})
				}()

			case <-time.After(90 * time.Second):
				go l.listener.Ping()

			case <-l.done:
				l.status = false
				return
			}
		}
	}()
}

func (l *pqListener) Name() string {
	return l.name
}

func (l *pqListener) Stop() {
	l.done <- true
}
