package database

import (
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"testing"
	"time"
)

func TestPoolChecker(*testing.T) {
	l := logger.New("\"DEBUG\"", "")
	p := NewPool(config.ConnectionsPool{
		AttemptsFail:   5,
		AttemptsPeriod: "5s",
		LockPeriod:     "30s",
	}, l)

	poolStatement := p.AddStatement(&statement{})
	for i := 0; i < 5; i++ {
		poolStatement.AddFailedAttempt()
		// time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(20 * time.Second)
}
