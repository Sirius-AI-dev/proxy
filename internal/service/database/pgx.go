package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"time"
)

type pgxPoolConnection struct {
	connection *pgxpool.Pool
	log        *logger.Logger
	timeout    time.Duration
	config     config.Connection
}

type pgxTransaction struct {
	tx pgx.Tx
}

func (tx *pgxTransaction) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

func (tx *pgxTransaction) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}

type pgxRows struct {
	rows pgx.Rows
}

type pgxResult struct {
	tx        *pgxTransaction
	result    pgconn.CommandTag
	rows      pgxRows
	statement Statement
	ctx       context.Context
	log       *logger.Logger
	data      *map[string]interface{}
	body      []byte
}

func (r *pgxResult) Apply(m *map[string]interface{}) error {
	return utils.MapMerge(r.data, m)
}

func (r *pgxResult) Set(m *map[string]interface{}) {
	r.data = m
}

func (r *pgxResult) Transaction() Transaction {
	return r.tx
}

func NewPgxPoolConnection(cfg config.Connection, cfgDB *config.Database, log *logger.Logger) Connection {
	var err error
	var dbConfig *pgxpool.Config
	dbConfig, err = pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		panic(err)
	}
	dbConfig.MinConns = 1
	if cfg.MaxOpenConn > 0 {
		dbConfig.MaxConns = cfg.MaxOpenConn
	}

	var connection *pgxpool.Pool
	connection, err = pgxpool.NewWithConfig(context.Background(), dbConfig)

	timeout := cfgDB.Defaults.Timeout
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	return &pgxPoolConnection{
		connection: connection,
		log:        log,
		timeout:    timeout,
		config:     cfg,
	}
}

func (s *pgxPoolConnection) NewNotifyListener(listenerName string, channel string) NotifyListener {
	panic("Can't create listener on pool")
}

func (s *pgxPoolConnection) Connection() interface{} {
	return s.connection
}

func (s *pgxPoolConnection) Status() bool {
	return s.connection.Stat().TotalConns() > 0
}

func (s *pgxPoolConnection) Timeout() time.Duration {
	return s.timeout
}

func (s *pgxPoolConnection) Config() *config.Connection {
	return &s.config
}

func (s *pgxPoolConnection) Query(ctx context.Context, sql string, args ...interface{}) (result QueryResult, err error) {
	args[0] = pgtype.JSONB{Bytes: args[0].([]byte), Status: pgtype.Status(2)}

	var tx pgx.Tx
	var pgxRowsRaw pgx.Rows

	if s.config.ReadOnly {
		// Replica behaviour (without transaction)
		pgxRowsRaw, err = s.connection.Query(ctx, sql, args...)
	} else {
		// Master behaviour
		tx, err = s.connection.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, err
		}
		pgxRowsRaw, err = tx.Query(ctx, sql, args...)
	}

	result = &pgxResult{
		ctx: ctx,
		log: s.log,
		tx: &pgxTransaction{
			tx: tx,
		},
		rows: pgxRows{
			rows: pgxRowsRaw,
		},
	}

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return //pgx.Rows
}

func (s *pgxPoolConnection) Exec(ctx context.Context, sql string, args ...interface{}) (result QueryResult, err error) {
	args[0] = pgtype.JSONB{Bytes: args[0].([]byte), Status: pgtype.Status(2)}

	res, err := s.connection.Exec(ctx, sql, args...)
	result = &pgxResult{
		ctx:    ctx,
		log:    s.log,
		result: res,
	}

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return
}

func (s *pgxPoolConnection) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}

	return &pgxTransaction{
		tx: tx,
	}, nil
}

func (r *pgxRows) Close() error {
	r.rows.Close()
	return nil
}

func (r *pgxRows) Err() error {
	return r.rows.Err()
}

func (r *pgxRows) Next() bool {
	return r.rows.Next()
}

func (r *pgxRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

type pgxConnection struct {
	config     config.Connection
	connection *pgx.Conn
	logger     *logger.Logger
	timeout    time.Duration
}

func NewPgxConnection(cfg config.Connection, cfgDB *config.Database, log *logger.Logger) Connection {
	var err error
	var dbConfig *pgx.ConnConfig
	dbConfig, err = pgx.ParseConfig(cfg.DSN)
	if err != nil {
		panic(err)
	}

	var connection *pgx.Conn
	connection, err = pgx.ConnectConfig(context.Background(), dbConfig)

	timeout := cfgDB.Defaults.Timeout
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	return &pgxConnection{
		config:     cfg,
		connection: connection,
		logger:     log,
		timeout:    timeout,
	}
}

func (s *pgxConnection) Connection() interface{} {
	return s.connection
}

func (s *pgxConnection) Status() bool {
	return !s.connection.PgConn().IsClosed()
}

func (s *pgxConnection) Timeout() time.Duration {
	return s.timeout
}

func (s *pgxConnection) Config() *config.Connection {
	return &s.config
}

func (s *pgxConnection) Query(ctx context.Context, sql string, args ...interface{}) (result QueryResult, err error) {
	args[0] = pgtype.JSONB{Bytes: args[0].([]byte), Status: pgtype.Status(2)}

	pgxRowsRaw, err := s.connection.Query(ctx, sql, args...)
	result = &pgxResult{
		rows: pgxRows{
			rows: pgxRowsRaw,
		},
	}

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return //pgx.Rows
}

func (s *pgxConnection) Exec(ctx context.Context, sql string, args ...interface{}) (result QueryResult, err error) {
	args[0] = pgtype.JSONB{Bytes: args[0].([]byte), Status: pgtype.Status(2)}

	res, err := s.connection.Exec(ctx, sql, args...)
	result = &pgxResult{
		result: res,
	}

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return
}

func (s *pgxConnection) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}

	return &pgxTransaction{
		tx: tx,
	}, nil
}

func (r *pgxResult) AffectedRows() int64 {
	affectedRows := r.result.RowsAffected()
	return affectedRows
}

func (r *pgxResult) LastInsertId() interface{} {
	return nil
}

func (r *pgxResult) Statement() Statement {
	return r.statement
}

func (r *pgxResult) SetStatement(statement Statement) {
	r.statement = statement
}

func (r *pgxResult) Rows() Rows {
	if r.rows.rows != nil {
		return &r.rows
	}
	return nil
}

func (r *pgxResult) MapData() (data map[string]interface{}, err error) {
	if r.Rows() != nil {
		var jResponse []byte
		var response map[string]interface{}

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

			// else commit transaction
			if err = r.tx.Commit(r.ctx); err != nil {
				r.log.Error(err)
			}
		}()

		if r.Rows().Next() {
			err = r.Rows().Scan(&jResponse)
			if err == nil {
				r.body = jResponse
				err = json.Unmarshal(jResponse, &response)
			}

			if err == nil {
				return response, nil
			}
		}
	}
	return nil, err
}

func (r *pgxResult) Body() []byte {
	if r.Rows() != nil {
		var jResponse []byte
		if r.Rows().Next() {
			err := r.Rows().Scan(&jResponse)

			if err == nil {
				return jResponse
			}
		}
	}
	return []byte{}
}

type pgxListener struct {
	name       string
	connection *pgx.Conn
	logger     *logger.Logger
	status     bool
}

func (s *pgxConnection) NewNotifyListener(listenerName string, channel string) NotifyListener {
	_, err := s.connection.Exec(context.Background(), fmt.Sprintf("listen %s", channel))
	if err != nil {
		return nil
	}

	return &pgxListener{
		name:       listenerName,
		connection: s.connection,
		logger:     s.logger,
		status:     true,
	}
}

func (l *pgxListener) Listen(notificator NotifyHandler) {
	go func() {
		for {
			if notification, err := l.connection.WaitForNotification(context.Background()); err != nil {
				l.logger.Debugf("NOTIFY on channel '%s' with payload: %s", notification.Channel, notification.Payload)
				go func() {
					notificator(Notification{
						Channel: notification.Channel,
						Payload: notification.Payload,
					})
				}()
			}

			if !l.status {
				return // break listener
			}
		}
	}()
}

func (l *pgxListener) Name() string {
	return l.name
}

func (l *pgxListener) Stop() {
	l.status = false
}
