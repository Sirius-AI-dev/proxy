package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/unibackend/uniproxy/internal/common"
	"net/http"
	"net/http/pprof"
)

func (s *HandleService) PProfHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()

	var pprofHandleFunc func(w http.ResponseWriter, r *http.Request)
	var pprofHandle http.Handler

	ppName := chi.URLParam(request, "ppName")

	switch ppName {
	case "":
		pprofHandleFunc = pprof.Index
	case "cmdline":
		pprofHandleFunc = pprof.Cmdline
	case "profile":
		pprofHandleFunc = pprof.Profile
	case "symbol":
		pprofHandleFunc = pprof.Symbol
	case "trace":
		pprofHandleFunc = pprof.Trace
	default:
		pprofHandle = pprof.Handler(ppName)
	}

	if pprofHandle != nil {
		pprofHandle.ServeHTTP(w, request)
	} else {
		pprofHandleFunc(w, request)
	}

	return response
}
