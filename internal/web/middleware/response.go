package middleware

import (
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
)

// ResponseMiddleware parses request
// "response" middleware
func ResponseMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
