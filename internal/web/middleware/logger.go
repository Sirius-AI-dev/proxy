package middleware

import (
	"context"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"net/http"
)

// LoggerMiddleware parses request
// "logger" middleware
func LoggerMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := params["logger"].(*logger.Logger).WithField("method", r.Method)
			log.AddField("host", r.Host)
			log.AddField("ip", r.RemoteAddr)
			log.AddField("url", r.RequestURI)

			r = r.WithContext(context.WithValue(r.Context(), common.LoggerKey, log))

			next.ServeHTTP(w, r)
		})
	}
}
