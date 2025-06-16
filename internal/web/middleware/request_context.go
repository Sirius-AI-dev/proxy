package middleware

import (
	"context"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
)

// RequestContextMiddleware parses request
// "request_context" middleware
func RequestContextMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			//log := r.Context().Value(common.LoggerKey).(*logger.Logger)
			requestContext := map[string]string{
				"host":   r.Host,
				"method": r.Method,
				"ip":     r.RemoteAddr,
				"url":    r.RequestURI,
			}

			if ua := r.Header.Get("User-Agent"); ua != "" {
				requestContext["ua"] = ua
			}

			if lang := r.Header.Get("Accept-Language"); lang != "" {
				requestContext["lang"] = lang
			}

			if ref := r.Referer(); ref != "" {
				requestContext["ref"] = ref
			}

			requestBody := r.Context().Value(common.RequestBodyKey).(*map[string]interface{})
			(*requestBody)[common.RequestContextKey] = requestContext
			r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, requestBody))
			r = r.WithContext(context.WithValue(r.Context(), common.RequestContextKey, requestContext))

			next.ServeHTTP(w, r)
		})
	}
}
