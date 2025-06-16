package middleware

import (
	"context"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"net/http"
	"strconv"
)

var modifierFunctions map[string]func(*http.Request, http.ResponseWriter, map[string]interface{}, *map[string]interface{})

func init() {
	modifierFunctions = make(map[string]func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}))

	registerModifiers()
}

func ModifyRequestMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			requestBody := r.Context().Value(common.RequestBodyKey).(*map[string]interface{})

			if modifiers, ok := params["modifiers"].([]interface{}); ok {
				for _, modifier := range modifiers {
					var modifierData map[string]interface{}

					// for json style config
					if mType, ok := modifier.(map[string]interface{}); ok {
						modifierData = mType

						// for yaml style config
					} else if mType, ok := modifier.(map[interface{}]interface{}); ok {
						modifierData = make(map[string]interface{})
						for k, v := range mType {
							modifierData[k.(string)] = v
						}
					}

					if modifierFunc := modifierFunctions[modifierData["modifierName"].(string)]; ok {
						modifierFunc(r, w, modifierData, requestBody)
					}
				}
			}
			r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, requestBody))

			next.ServeHTTP(w, r)
		})
	}
}

func registerModifiers() {

	// Group the set of fields (first level) of requestBody to another specified field (first level) of requestBody
	modifierFunctions["map-request-field"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		rBody := *requestBody
		mapping := map[string]map[string]interface{}{}
		mappingValue := map[string]interface{}{}
		for key, toKey := range modifierData {
			if key == "modifierName" {
				continue
			}
			if _, ok := rBody[key]; !ok { // Continue, if field is not present on requestBody
				continue
			}

			mappingValue[toKey.(string)] = rBody[key]
			delete(rBody, key)
		}
		for k, v := range mapping {
			rBody[k] = v
		}
		for k, v := range mappingValue {
			rBody[k] = v
		}
		requestBody = &rBody // new pointer
	}

	// Set fields to the requestBody fields from request headers
	modifierFunctions["header-to-request"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		rBody := *requestBody
		for key, toKey := range modifierData {
			if key == "modifierName" {
				continue
			}
			if value := request.Header.Get(key); value != "" {
				rBody[toKey.(string)] = value
			}
		}
		requestBody = &rBody // new pointer
	}

	// Set response headers by specified map fields
	modifierFunctions["header-response"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		for key, value := range modifierData {
			if key == "modifierName" {
				continue
			}
			response.Header().Add(key, value.(string))
		}
	}

	// Set request headers by specified map fields
	modifierFunctions["header-request"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		for key, value := range modifierData {
			if key == "modifierName" {
				continue
			}
			request.Header.Set(key, value.(string))
		}
	}

	modifierFunctions["cookie-to-request"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		rBody := *requestBody
		for key, value := range modifierData {
			if key == "modifierName" {
				continue
			}

			if cookieValue, err := request.Cookie(key); err == nil && cookieValue.Value != "" {
				rBody[value.(string)] = cookieValue.Value
			}
		}
		requestBody = &rBody // new pointer
	}

	// Type asserting of values
	modifierFunctions["type"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		rBody := *requestBody
		for key, typeField := range modifierData {
			if key == "modifierName" {
				continue
			}
			value := rBody[key]
			switch value.(type) {
			case int:
				switch typeField {
				case "string":
					rBody[key] = strconv.Itoa(value.(int))
				}

			case string:
				switch typeField {
				case "int":
					if val, err := strconv.Atoi(value.(string)); err == nil {
						rBody[key] = val
					}
				}
			}
		}
		requestBody = &rBody // new pointer
	}

	// Extends
	modifierFunctions["extends"] = func(request *http.Request, response http.ResponseWriter, modifierData map[string]interface{}, requestBody *map[string]interface{}) {
		rBody := *requestBody
		if fieldName, ok := modifierData["fieldName"].(string); ok {
			for key, value := range rBody[fieldName].(map[string]interface{}) {
				if key == "field" {
					continue
				}
				rBody[key] = value
			}
			delete(rBody, fieldName)
			requestBody = &rBody // new pointer
		}
	}
}
