package middleware

import (
	"context"
	"github.com/avct/uasurfer"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
)

const MobileDetectKey = "mobiledetect"

func MobileDetectMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ua := uasurfer.Parse(r.UserAgent())
			requestBody := *r.Context().Value(common.RequestBodyKey).(*map[string]interface{})
			requestBody[MobileDetectKey] = ua
			r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, requestBody))

			next.ServeHTTP(w, r)
		})
	}
}
