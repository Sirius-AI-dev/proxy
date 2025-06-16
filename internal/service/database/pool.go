package database

import (
	"context"
	"github.com/jackc/pgconn"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"sync"
	"time"
)

const (
	checkInterval = 200 * time.Millisecond
)

// poolStatement structure need for represents statement in pool
// with own stats for failAttempts on pool connections
type poolStatement struct {
	statement         Statement
	failAttempts      []time.Time
	failAttemptsMutex *sync.Mutex
	pool              *pool
}

type pool struct {
	statements         []*poolStatement
	failAttemptsPeriod time.Duration
	failAttemptsCount  int
	lockPeriod         time.Duration
	defaultConnection  string
	l                  *logger.Logger
	timeout            time.Duration
	cfgPool            config.ConnectionsPool
}

type Pool interface {
	AddStatement(Statement) *poolStatement
	Query(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
	Exec(context.Context, *logger.Logger, ...interface{}) (QueryResult, error)
}

func NewPool(cfgPool config.ConnectionsPool, l *logger.Logger) Pool {
	attemptsPeriod, err := time.ParseDuration(cfgPool.AttemptsPeriod)
	if cfgPool.AttemptsPeriod != "" && err != nil {
		panic(err)
	}

	lockPeriod, err := time.ParseDuration(cfgPool.LockPeriod)
	if cfgPool.LockPeriod != "" && err != nil {
		panic(err)
	}

	p := &pool{
		statements:         make([]*poolStatement, 0),
		failAttemptsPeriod: attemptsPeriod,
		failAttemptsCount:  cfgPool.AttemptsFail,
		lockPeriod:         lockPeriod,
		defaultConnection:  cfgPool.DefaultConnection,
		l:                  l,
		timeout:            cfgPool.Timeout,
	}

	go p.checker()

	return p
}

// checker checks all connections in pool for locking state
func (p *pool) checker() {
	for {
		now := time.Now()
		for _, stStruct := range p.statements {
			if p.failAttemptsCount == 0 {
				continue
			}

			// If in the last "failattemptsperiod" there were "failattempts" failed query attempts, then lock the statement for "lockperiod"
			if !stStruct.statement.Locked() && len(stStruct.failAttempts) >= p.failAttemptsCount {
				stStruct.statement.Lock(p.lockPeriod)
				p.l.Debugf("Lock statement: %s on %s", stStruct.statement.Id(), p.lockPeriod.String())
			}

			// Release expired fail attempts
			for i := 0; i < len(stStruct.failAttempts); i++ { //i, attempt := range stStruct.failAttempts {
				if stStruct.failAttempts[i].Before(now) {
					p.l.Debugf("Release lock statement: %s on %s", stStruct.statement.Id(), stStruct.failAttempts[i].String())
					stStruct.failAttemptsMutex.Lock()
					stStruct.failAttempts = append(stStruct.failAttempts[:i], stStruct.failAttempts[i+1:]...)
					stStruct.failAttemptsMutex.Unlock()
				}
			}
		}

		time.Sleep(checkInterval)
	}
}

func (p *pool) AddStatement(statement Statement) *poolStatement {
	pStatement := &poolStatement{
		statement:         statement,
		failAttempts:      make([]time.Time, 0),
		failAttemptsMutex: &sync.Mutex{},
		pool:              p,
	}
	p.statements = append(p.statements, pStatement)
	return pStatement
}

func (p *pool) Query(ctx context.Context, logger *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	for _, queryStatement := range p.statements {
		// We can execute query on this statement because his not locked
		if !queryStatement.statement.Locked() {
			//p.l.Debugf("Execute statement: %s", statement.statement.Id())
			if result, err = p.query(ctx, queryStatement, logger, args...); err == nil {
				return
			}
		}
	}
	return
}

func (p *pool) query(ctx context.Context, pStatement *poolStatement, logger *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	//ctx, cancel := context.WithTimeout(ctx, statement.statement.Timeout())
	//defer cancel()
	if result, err = pStatement.statement.Query(ctx, logger, args...); err != nil {
		logger.Error(err)
		if pgconn.Timeout(err) && pStatement.statement.ConnId() != p.defaultConnection {
			pStatement.AddFailedAttempt()
		}
		return
	}
	result.SetStatement(pStatement.statement)
	return
}

func (p *pool) Exec(ctx context.Context, logger *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	for _, pStatement := range p.statements {
		// We can execute query on this statement because his not locked
		if !pStatement.statement.Locked() {
			//p.l.Debugf("Execute statement: %s", statement.statement.Id())
			if result, err = p.exec(ctx, pStatement, logger, args...); err == nil {
				return
			}
		}
	}
	return
}

func (p *pool) exec(ctx context.Context, pStatement *poolStatement, logger *logger.Logger, args ...interface{}) (result QueryResult, err error) {
	if result, err = pStatement.statement.Exec(ctx, logger, args...); err != nil {
		logger.Error(err)
		if pgconn.Timeout(err) && pStatement.statement.ConnId() != p.defaultConnection {
			pStatement.AddFailedAttempt()
		}
		return
	}
	result.SetStatement(pStatement.statement)
	return
}

func (d *db) DefaultPool() Pool {
	return d.Pool(d.config.Defaults.Pool)
}

func (ps *poolStatement) AddFailedAttempt() {
	ps.failAttemptsMutex.Lock()
	ps.failAttempts = append(ps.failAttempts, time.Now().Add(ps.pool.failAttemptsPeriod))
	ps.failAttemptsMutex.Unlock()
	ps.pool.l.Debugf("+1 fail attempt: %s", ps.statement.Id())
}
