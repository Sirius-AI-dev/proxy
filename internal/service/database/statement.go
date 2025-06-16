package database

import (
	"context"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"time"
)

func (d *db) RegisterStatement(name string, stmt Statement) {
	d.statements[name] = stmt
}

func (d *db) Statement(name string) Statement {
	if statement, ok := d.statements[name]; ok {
		return statement
	}
	d.log.Errorf("Statement '%s' not found", name)
	return nil
}

func (d *db) DefaultStatement() Statement {
	return d.Statement(d.config.Defaults.Statement)
}

type Statement interface {
	Query(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
	Exec(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
	Id() string
	ConnId() string
	Lock(time.Duration)
	LockUntil(time.Time)
	Locked() bool
	Timeout() time.Duration
}

type statement struct {
	conn        Connection
	sql         string
	log         *logger.Logger
	id          string
	connId      string
	measurer    metrics.Measurer
	timeout     time.Duration
	lockedUntil time.Time
}

func NewStatement(
	conn Connection,
	log *logger.Logger,
	sql string,
	id string,
	connId string,
	measurer metrics.Measurer,
	timeout time.Duration,
) Statement {
	return &statement{
		conn:     conn,
		sql:      sql,
		log:      log,
		id:       id,
		connId:   connId,
		measurer: measurer,
		timeout:  timeout,
	}
}

func (s *statement) Id() string {
	return s.id
}
func (s *statement) ConnId() string {
	return s.connId
}

func (s *statement) Query(ctx context.Context, log *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	ctx = context.WithValue(ctx, "statementId", s.id)
	result, err = s.conn.Query(ctx, s.sql, args...)

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return
}

func (s *statement) Exec(ctx context.Context, log *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	ctx = context.WithValue(ctx, "statementId", s.id)
	result, err = s.conn.Exec(ctx, s.sql, args...)

	// Error by context cancelled or timeout
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return
}

func (s *statement) Lock(duration time.Duration) {
	s.lockedUntil = time.Now().Add(duration)
}
func (s *statement) LockUntil(t time.Time) {
	s.lockedUntil = t
}

func (s *statement) Locked() bool {
	return s.lockedUntil.After(time.Now())
}

func (s *statement) Timeout() time.Duration {
	return s.timeout
}
