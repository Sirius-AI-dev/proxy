package database

import (
	"context"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"time"
)

type Connection interface {
	Query(context.Context, string, ...interface{}) (QueryResult, error)
	Exec(context.Context, string, ...interface{}) (QueryResult, error)
	NewNotifyListener(listenerName string, channel string) NotifyListener
	Connection() interface{}
	Config() *config.Connection
	Status() bool
	Timeout() time.Duration
	BeginTx(ctx context.Context) (Transaction, error)
}

type Rows interface {
	Close() error

	Err() error

	Next() bool

	Scan(dest ...interface{}) error
}

type Transaction interface {
	Commit(context.Context) error
	Rollback(context.Context) error
}

type QueryResult interface {
	common.WithData
	AffectedRows() int64
	LastInsertId() interface{}
	Transaction() Transaction
	Rows() Rows

	Statement() Statement
	SetStatement(Statement)

	//MapData() (*map[string]interface{}, error)
	//Body() []byte
	//ProxyTasks() []types.Task
}
