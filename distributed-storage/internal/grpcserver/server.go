package grpcserver

import (
	"fmt"
	"net"
	"sync"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"google.golang.org/grpc"
)

// Server wraps grpc.Server with consistent logging and graceful shutdown hooks.
type Server struct {
	addr       string
	grpcServer *grpc.Server
	log        *logger.Logger
	stopOnce   sync.Once // sync.Once for grpc server graceful shutdown so Stop is idempotent
}

// NewServer builds a grpc server with optional extra options for tls in phase 3.
func NewServer(addr string, log *logger.Logger, opts ...grpc.ServerOption) *Server {
	return &Server{
		addr:       addr,
		grpcServer: grpc.NewServer(opts...),
		log:        log.WithComponent("grpc-server"),
	}
}

// GRPC returns the raw server so callers can register service implementations.
func (s *Server) GRPC() *grpc.Server {
	return s.grpcServer
}

// ListenAndServe blocks serving until Stop is called or listen errors.
func (s *Server) ListenAndServe() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpcserver.ListenAndServe: listen %s: %w", s.addr, err)
	}
	s.log.Info("grpc server listening", "addr", s.addr)
	return s.grpcServer.Serve(lis)
}

// Stop gracefully stops accepting new rpcs and waits for in-flight ones to finish.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.log.Info("grpc server stopping", "addr", s.addr)
		s.grpcServer.GracefulStop()
	})
}
