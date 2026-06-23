package grpcserver

import (
	"fmt"
	"sync"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	storagev1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/storagev1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientPool maintains a pool of grpc connections to storage nodes.
// dialing is expensive; reusing connections cuts latency significantly under load
type ClientPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn // nodeID -> connection
	log   *logger.Logger
}

// NewClientPool creates an empty lazy-dial connection pool.
func NewClientPool(log *logger.Logger) *ClientPool {
	return &ClientPool{
		conns: make(map[string]*grpc.ClientConn),
		log:   log.WithComponent("grpc-pool"),
	}
}

// GetClient returns a client for a node, dialing lazily on first access.
// lazy dial means startup isn't blocked waiting for all nodes to be ready
func (p *ClientPool) GetClient(nodeID, address string) (storagev1.StorageServiceClient, error) {
	p.mu.RLock()
	conn, ok := p.conns[nodeID]
	p.mu.RUnlock()
	if ok {
		return storagev1.NewStorageServiceClient(conn), nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok = p.conns[nodeID]; ok {
		return storagev1.NewStorageServiceClient(conn), nil
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcserver.ClientPool.GetClient: dial %s (%s): %w", nodeID, address, err)
	}
	p.conns[nodeID] = conn
	p.log.Info("pooled storage connection", "node_id", nodeID, "address", address)
	return storagev1.NewStorageServiceClient(conn), nil
}

// Close shuts down all pooled connections on master shutdown.
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, conn := range p.conns {
		_ = conn.Close()
		delete(p.conns, id)
	}
}

// Dial connects to a remote grpc endpoint with insecure creds for local docker-compose.
func Dial(addr string, log *logger.Logger, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	base = append(base, opts...)

	conn, err := grpc.NewClient(addr, base...)
	if err != nil {
		return nil, fmt.Errorf("grpcserver.Dial: %s: %w", addr, err)
	}
	log.WithComponent("grpc-client").Info("connected", "addr", addr)
	return conn, nil
}
