package web

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/internal/task"
	"github.com/unibackend/uniproxy/internal/web/handler"
	"github.com/unibackend/uniproxy/internal/web/middleware"
	"github.com/unibackend/uniproxy/utils"
	"net/http"
	"strings"
	"time"
)

const ServiceName = "server"

const requestIdHeaderKey = "X-Request-Id"
const requestIdLoggerKey = "request_id"

var (
	specialKeysOfResponse = []string{"proxyTasks", "proxyRollback"}
	json                  = jsoniter.ConfigCompatibleWithStandardLibrary
)

type Server interface {
	service.Interface
	// AddRoute builds a route handler and adds it to the node tree of the server router
	// The handler also wraps up the required and optional middleware for additional request/response processing
	AddRoute(r *chi.Mux, name string, route *config.Route)

	GetRouter() *chi.Mux

	ApplyConfig(*config.Server) error
}

type webService struct {
	log            *logger.Logger
	router         *chi.Mux
	config         *config.Server
	taskService    task.Service
	handlerService *handler.HandleService
	middlewares    map[string]func(map[string]interface{}, *config.Route) func(next http.Handler) http.Handler
}

func New(
	log *logger.Logger,
	cfg *config.Server,
	tasks task.Service,
	handlerService *handler.HandleService,

) Server {

	h := &webService{
		log:            log.WithField(logger.ServiceKey, ServiceName),
		taskService:    tasks,
		config:         cfg,
		handlerService: handlerService,
		middlewares:    map[string]func(map[string]interface{}, *config.Route) func(next http.Handler) http.Handler{},
	}

	h.registerMiddlewares()

	//if err := h.ApplyConfig(cfg); err != nil {
	//	log.Error(err)
	//	panic(err)
	//}

	return h
}

func (s *webService) ApplyConfig(server *config.Server) error {
	// Apply main configs
	s.config.Timeout = server.Timeout
	s.config.JwtSecret = server.JwtSecret
	s.config.Cors = server.Cors

	s.config.Cors.HeadersString = strings.Join(s.config.Cors.Headers, ",")

	if len(s.config.Middleware.Required) == 0 && len(server.Middleware.Required) > 0 {
		s.config.Middleware.Required = server.Middleware.Required
	}

	// Apply domains
	if len(server.Domains) > 0 {
		for _, domain := range server.Domains {
			// Check domain if exists
			if !utils.InArrayString(s.config.Domains, domain) {
				s.config.Domains = append(s.config.Domains, domain)
			}
		}

		s.log.Infof("Set domains: %s", strings.Join(s.config.Domains, ", "))
	}

	s.config.Routes = server.Routes

	// Load routes
	if s.config.Routes == nil {
		s.config.Routes = make(map[string]*config.Route)
	}

	for routeName, route := range server.Routes {
		if _, ok := s.config.Routes[routeName]; !ok {
			s.config.Routes[routeName] = route
		}
	}

	router := chi.NewRouter()

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response := s.handlerService.NotFoundHandler(r, w)
		log := r.Context().Value(common.LoggerKey).(*logger.Logger)
		log.Infof("Endpoint not found")
		sendError(s.log, w, response.Status(), response.Error().Error(), response)
	})

	// Config routes from file
	for name, route := range s.config.Routes {
		s.AddRoute(router, name, route)
	}

	s.router = router

	return nil
}

func (s *webService) GetRouter() *chi.Mux {
	return s.router
}

func (s *webService) AddRoute(parentRouter *chi.Mux, routeName string, route *config.Route) {
	router := chi.NewRouter()

	// Allow methods on endpoint
	methods := make([]string, len(route.Methods))
	for key, method := range route.Methods {
		methods[key] = strings.ToUpper(method)
	}
	methods = append(methods, http.MethodOptions)

	if len(route.Domains) == 0 {
		route.Domains = s.config.Domains
	}

	// Required middlewares
	timeout := utils.ParseDuration(route.Timeout, 30*time.Second)

	router.Use(s.middlewares["logger"](map[string]interface{}{
		"logger": s.log,
	}, route))
	router.Use(chimw.Timeout(timeout))
	router.Use(chimw.RequestID)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://*", "https://*"}, //s.config.Cors.Domains, // can be []string {"http://*", "https://*"}
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods: route.Methods,
		AllowedHeaders: s.config.Cors.Headers,
		//ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	router.Use(chimw.RealIP)

	router.Use(chimw.Recoverer)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "router", router)

	// createRouteHandler called on handle of request from ServeHTTP
	var routeHandler http.HandlerFunc = s.createRouteHandler(ctx, routeName, route)

	// Required middlewares
	for _, mw := range s.config.Middleware.Required {
		if mware, ok := s.middlewares[mw["name"].(string)]; ok {
			router.Use(mware(mw, route))
		}
	}

	// Optional middlewares
	if route.Middlewares != nil {
		for _, mw := range route.Middlewares {
			if mware, ok := s.middlewares[mw["name"].(string)]; ok {
				router.Use(mware(mw, route))
			}
		}
	}

	// Register handler in router
	for _, endpoint := range route.Endpoints {
		for _, method := range methods {
			switch strings.ToLower(method) {
			case "head":
				router.Head("/", routeHandler)
			case "post":
				router.Post("/", routeHandler)
			case "put":
				router.Put("/", routeHandler)
			case "patch":
				router.Patch("/", routeHandler)
			case "delete":
				router.Delete("/", routeHandler)
			default: // included GET
				router.Get("/", routeHandler)

			}
		}
		parentRouter.Mount(endpoint, router)
	}

	s.log.Debugf("Route %s [%s] (internal: %v)", strings.Join(route.Endpoints, ", "), strings.Join(route.Methods, ", "), route.Internal)
}

func (s *webService) createRouteHandler(ctx context.Context, routeName string, route *config.Route) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Set route config and router of the path of endpoint to request context
		ctx = context.WithValue(r.Context(), "route", route)
		r = r.WithContext(ctx)

		log := r.Context().Value(common.LoggerKey).(*logger.Logger)

		// routeContext := r.Context().Value(chi.RouteCtxKey).(chi.Context)
		requestId := r.Context().Value(chimw.RequestIDKey)
		if requestId != nil {
			log.AddField(requestIdLoggerKey, requestId.(string))
		}

		log.AddField("route", routeName)

		log.Infof("Receive request")

		funcHandler := s.handlerService.GetHandler(route.Handler)

		// Handler is not defined and specified tasks
		if route.Handler == "" && len(route.Tasks) > 0 {
			funcHandler = s.handlerService.DefaultHandler
		}

		// Internal route and is public request
		if route.Internal && utils.InArrayString(route.Domains, r.Host) {
			funcHandler = s.handlerService.NotFoundHandler
		}

		if funcHandler == nil && (route.Handler != "" || len(route.Tasks) == 0) {
			log.Errorf("handler '%s' on route '%s' not found or no tasks", route.Handler, routeName)
			sendError(log, w, http.StatusNotFound, fmt.Sprintf("handler '%s' not found or no tasks", route.Handler), nil)
			return
		}

		begin := time.Now() // Fix begin time of calling service

		// Call handler
		response := funcHandler(r, w)

		s.taskService.GetService(metrics.ServiceName).(metrics.Measurer).SummaryVec(metrics.ProxyHandlerDuration).With(prometheus.Labels{
			"host":    r.Host,
			"route":   route.Endpoints[0],
			"handler": route.Handler,
			"status":  fmt.Sprintf("%d", response.Status()),
		}).Observe(float64(time.Since(begin)) / float64(time.Millisecond))

		if response.Status() >= http.StatusBadRequest {
			log.Error(response.Error())
			sendError(log, w, response.Status(), response.Error().Error(), response)
			return
		}

		sendResponse(log, w, response)
		return
	}
}

func sendResponse(log *logger.Logger, w http.ResponseWriter, response common.HandlerResponse) {

	// Remove special keys from response
	for _, key := range specialKeysOfResponse {
		response.RemoveKey(key)
	}

	// Write headers to response
	writeHeaders(w, response)

	if response.Status() != http.StatusOK {
		w.WriteHeader(response.Status())
	}

	// Write and sending response to client
	if _, err := w.Write(response.Body()); err != nil {
		log.Error(err)
	}
}

func sendError(log *logger.Logger, w http.ResponseWriter, code int, message string, response common.HandlerResponse) {
	writeHeaders(w, response)

	w.WriteHeader(code)
	errorStruct := map[string]interface{}{
		"status": "error",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	errorStructMarshaled, _ := json.Marshal(errorStruct)
	if _, err := w.Write(errorStructMarshaled); err != nil {
		log.Error(err)
	}
}

func writeHeaders(w http.ResponseWriter, response common.HandlerResponse) {
	// Write headers to response
	if response != nil {
		for headerKey, headerArray := range *response.Headers() {
			for _, headerValue := range headerArray {
				w.Header().Add(headerKey, headerValue)
			}
		}
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
	}
}

func (s *webService) Do(task common.Task) service.Result {
	return nil
}

func (s *webService) registerMiddlewares() {
	// Register middlewares for use in call services
	s.middlewares["request"] = middleware.RequestMiddleware
	s.middlewares["response"] = middleware.ResponseMiddleware
	s.middlewares["logger"] = middleware.LoggerMiddleware
	s.middlewares["request_context"] = middleware.RequestContextMiddleware
	s.middlewares["modify_request"] = middleware.ModifyRequestMiddleware
	s.middlewares["mobile_detect"] = middleware.MobileDetectMiddleware

	s.middlewares["user"] = middleware.AuthMiddleware
	s.middlewares["auth"] = middleware.AuthMiddleware
}
