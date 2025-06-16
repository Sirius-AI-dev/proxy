package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/grpc"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/internal/web"
	"github.com/unibackend/uniproxy/internal/worker"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const serverWaitSeconds = 5
const ServiceName = "proxy"

const KeyOperation = "operation"
const OperationReloadRouter = "reload_router"

type proxyResult struct {
	code    int
	headers http.Header
	body    []byte
	err     error
}

func (r *proxyResult) Code() int {
	return r.code
}

func (r *proxyResult) Headers() *http.Header {
	return &r.headers
}

func (r *proxyResult) Body() []byte {
	return r.body
}

func (r *proxyResult) Error() error {
	return r.err
}

func (r *proxyResult) MapData() (mapBody map[string]interface{}, err error) {
	return nil, nil
}

func (r *proxyResult) Apply(data *map[string]interface{}) error {
	return nil
}

func (r *proxyResult) Set(data *map[string]interface{}) {
}

func (r *proxyResult) Tasks() []common.Task {
	return []common.Task{}
}

type Proxy struct {
	service.Interface
	config   *config.Config
	log      *logger.Logger
	measurer metrics.Measurer

	webServer  web.Server
	httpServer *http.Server
}

func New(
	cfg *config.Config,
	log *logger.Logger,
) *Proxy {

	// Metrics
	measurer := metrics.New(cfg.Metrics, log)

	return &Proxy{
		config:   cfg,
		log:      log,
		measurer: measurer,
	}
}

func (c *Container) RunProxy() error {
	// Run DI container
	if err := c.container.Invoke(func(proxy *Proxy, webServer web.Server, workerService worker.Interface, gRPCServer grpc.Server) error {

		go func() {
			gRPCServer.Run()
		}()

		// Run worker in background
		go func() {
			if err := workerService.Run(); err != nil {
				panic(err)
			}
		}()

		// Run proxy listen
		return proxy.Listen(webServer)
	}); err != nil {
		return err
	}

	return nil
}

func (p *Proxy) Listen(webServer web.Server) error {
	p.webServer = webServer
	// Listen OS signal to prevent to stop server
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		osCall := <-stop
		p.log.Infof("System call: %+v", osCall)
		cancel()
	}()

	if err := p.serve(ctx); err != nil {
		p.log.Errorf("Failed to serve: +%v\n", err)
	}

	return nil
}

func (p *Proxy) serve(ctx context.Context) (err error) {
	p.httpServer = &http.Server{
		Handler: p.webServer.GetRouter(),
		Addr:    fmt.Sprintf("%s:%d", p.config.Server.HTTP.Host, p.config.Server.HTTP.Port),
	}

	go func() {
		if err := p.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			p.log.Error(err)
			panic(err)
		}
	}()

	p.log.Infof("HTTP server listening on %s", p.httpServer.Addr)

	// Block to stop server while context is existing
	<-ctx.Done()

	p.log.Info("Stop server...")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), serverWaitSeconds*time.Second)
	defer func() {
		cancel()
	}()

	// TODO: disconnect all es connections

	if err = p.httpServer.Shutdown(ctxShutDown); err != nil {
		panic(err)
	}

	p.log.Info("Server exited properly")

	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}

	return
}

func (p *Proxy) Do(task common.Task) service.Result {
	serviceResult := &proxyResult{
		code:    200,
		headers: http.Header{},
	}

	data, err := task.MapData()
	if err != nil {
		serviceResult.err = err
		return serviceResult
	}

	operation, ok := data[KeyOperation]
	if !ok {
		return serviceResult
	}

	if operation.(string) == OperationReloadRouter {
		p.httpServer.Handler = p.webServer.GetRouter()
	}

	return serviceResult
}
