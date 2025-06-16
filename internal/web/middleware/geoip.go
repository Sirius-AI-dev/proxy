package middleware

import (
	"context"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/service/geoip"
	"net"
	"net/http"
)

var geoIpService geoip.Service

// "geoip" middleware
func GeoIpMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {

	if geoIpService == nil {
		geoIpService = geoip.New(params["path"].(string))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if record, err := geoIpService.Record(net.ParseIP(r.RemoteAddr)); err == nil {
				rec := make(map[string]interface{})

				rec["city"] = record.City.Names["en"]
				rec["countryCode"] = record.Country.IsoCode
				rec["timezone"] = record.Location.TimeZone

				requestBody := *r.Context().Value(common.RequestBodyKey).(*map[string]interface{})
				requestBody["geoip"] = rec
				r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, requestBody))

			}

			next.ServeHTTP(w, r)
		})
	}
}
