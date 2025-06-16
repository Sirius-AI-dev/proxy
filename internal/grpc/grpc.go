package grpc

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/utils"
	"google.golang.org/grpc"
	"net"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Server interface {
	Run()
}

type grpcServer struct {
	log *logger.Logger
	cfg config.GRPCServer

	timeout time.Duration
}

func New(
	log *logger.Logger,
	cfg config.GRPCServer,
) Server {
	return &grpcServer{
		log: log,
		cfg: cfg,

		timeout: utils.ParseDuration(cfg.Timeout, time.Second*30),
	}
}

func (h *grpcServer) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", h.cfg.Host, h.cfg.Port))
	if err != nil {
		h.log.Error("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	registerService(server, &grpcService{})

	h.log.Infof("gRPC server listening on %s:%d", h.cfg.Host, h.cfg.Port)
	if err = server.Serve(listener); err != nil {
		h.log.Error("Failed to serve: %v", err)
	}
}
