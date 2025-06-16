package logger

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4"
	"log/slog"
	"os"
	"sync"
)

const (
	ServiceKey = "service"
)

type Logger struct {
	log    *slog.Logger
	fields map[string]string
	mu     sync.Mutex // Мьютекс для синхронизации доступа к fields
}

func New(level string, handler string) *Logger {
	var logLevel slog.Level //new(slog.LevelVar)

	var strLevel []byte

	if envLevel, ok := os.LookupEnv("LOG_LEVEL"); ok {
		strLevel = []byte(envLevel)
	} else if level != "" {
		strLevel = []byte(level)
	}

	if err := logLevel.UnmarshalJSON(strLevel); err != nil {
		panic(err)
	}

	var log *slog.Logger
	switch handler {
	case "text":
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	default:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	}

	return &Logger{
		log:    log,
		fields: make(map[string]string),
	}
}

func (l *Logger) GetLogger() *slog.Logger {
	return l.log
}

func (l *Logger) AddField(key string, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.log = l.log.With(key, value)
	l.fields[key] = value
}

func (l *Logger) AddFields(fields map[string]string) {
	log := l.log
	f := (*l).GetFields()
	for k, v := range fields {
		if _, ok := f[k]; !ok {
			log = log.With(k, v)
			f[k] = v
		}
	}
	l.log = log
}

func (l *Logger) GetFields() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.fields
}

func (l *Logger) WithField(key string, value string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	fields := make(map[string]string, len(l.fields)+1)
	for k, v := range l.fields {
		fields[k] = v
	}
	fields[key] = value

	return &Logger{
		log:    l.log.With(key, value),
		fields: fields,
		//mu:     l.mu,
	}
}

func (l *Logger) Debug(args ...interface{}) {
	l.log.Debug(fmt.Sprintf("%v", args[0]))
}
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log.Debug(fmt.Sprintf(format, args...))
}

func (l *Logger) Info(args ...interface{}) {
	l.log.Info(fmt.Sprintf("%v", args[0]))
}
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}

func (l *Logger) Error(args ...interface{}) {
	l.log.Error(fmt.Sprintf("%v", args[0]))
}
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	log := l.log
	for k, v := range data {
		log = log.With(k, v)
	}

	switch level {
	case pgx.LogLevelTrace:
		log.Log(context.Background(), slog.LevelDebug-1, msg, "PGX_LOG_LEVEL", level)
	case pgx.LogLevelDebug:
		log.Debug(msg)
	case pgx.LogLevelInfo:
		log.Info(msg)
	case pgx.LogLevelWarn:
		log.Warn(msg)
	case pgx.LogLevelError:
		log.Error(msg)
	default:
		log.Error(msg, "INVALID_PGX_LOG_LEVEL", level)
	}
}
