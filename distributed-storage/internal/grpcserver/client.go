package grpcserver

import (
	"fmt"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial connects to a remote grpc endpoint with insecure creds for local docker-compose.
// phase 3 swaps this for tls credentials from golang.org/x/crypto
func Dial(addr string, log *logger.Logger, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// prepend insecure transport so callers can't accidentally forget creds in dev
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	base = append(base, opts...)

	conn, err := grpc.NewClient(addr, base...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	log.WithComponent("grpc-client").Info("connected", "addr", addr)
	return conn, nil
}
