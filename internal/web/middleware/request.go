package middleware

import (
	"bytes"
	"context"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var (
	domains      []string
	excludedKeys = []string{"userId", "apicall"}
	json         = jsoniter.ConfigCompatibleWithStandardLibrary
)

// RequestMiddleware parses request
// "request" middleware
func RequestMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {

	if domainItems, ok := params["domains"]; ok {
		if domainItemsArray, ok := domainItems.([]string); ok {
			for _, domainItem := range domainItemsArray {
				domains = append(domains, domainItem)
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := r.Context().Value(common.LoggerKey).(*logger.Logger)
			requestBody := map[string]interface{}{}
			// Attempt parse request as json
			body, err := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset

			if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				if len(body) > 0 && err == nil {
					if err = json.Unmarshal(body, &requestBody); err != nil {
						log.Errorf("JSON unmarshal error %v", err)
					}
				}
			}

			if err := r.ParseForm(); err == nil {
				// Filling from Form data
				for key, value := range r.Form {
					requestBody[key] = value[0]
				}
			}

			if err := r.ParseMultipartForm(1 << 20); err == nil {
				// Filling from Form data
				for key, value := range r.Form {
					requestBody[key] = value[0]
				}
			}

			if errClose := r.Body.Close(); errClose != nil {
				log.Error(errClose)
				return
			}

			queryGet := r.URL.Query()
			for key, values := range queryGet {
				if len(values) == 1 {
					// Is numeric type?
					if i, err := strconv.Atoi(values[0]); err == nil {
						requestBody[key] = i
					} else {
						requestBody[key] = values[0]
					}
				} else {
					requestBody[key] = values // values is []string
				}
			}

			// Cut internal keys from request if call uniproxy by allowed domain
			if utils.InArrayString(domains, r.Host) {
				for _, key := range excludedKeys {
					delete(requestBody, key)
				}
			}

			// Set requestBody to context
			r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, &requestBody))

			next.ServeHTTP(w, r)
		})
	}
}
