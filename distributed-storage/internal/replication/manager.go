package replication

import (
	"context"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// Manager decides where replicas land and orchestrates copy/delete during failures.
// interface keeps master wiring testable with fakes before we build the real grpc fan-out
type Manager interface {
	// SelectNodes picks up to replicationFactor alive nodes with enough free space for a chunk
	SelectNodes(ctx context.Context, chunkSize int64, replicationFactor int) ([]domain.StorageNode, error)
	// ReplicateChunk copies chunk bytes to the chosen replica nodes after initial placement
	ReplicateChunk(ctx context.Context, chunk domain.Chunk, targets []domain.StorageNode) error
	// HandleNodeDeath schedules re-replication for chunks that lost a replica on a dead node
	HandleNodeDeath(ctx context.Context, deadNodeID string) error
}

// NoopManager is a phase-1 stub that satisfies the interface until grpc replication lands.
type NoopManager struct{}

// SelectNodes returns empty slice in scaffold; phase 2 will implement placement heuristics.
func (NoopManager) SelectNodes(ctx context.Context, chunkSize int64, replicationFactor int) ([]domain.StorageNode, error) {
	_ = ctx
	_ = chunkSize
	_ = replicationFactor
	return nil, nil
}

// ReplicateChunk is a no-op until storage nodes can receive ReplicateChunk rpcs.
func (NoopManager) ReplicateChunk(ctx context.Context, chunk domain.Chunk, targets []domain.StorageNode) error {
	_ = ctx
	_ = chunk
	_ = targets
	return nil
}

// HandleNodeDeath is a no-op until the heartbeat monitor wires into this manager.
func (NoopManager) HandleNodeDeath(ctx context.Context, deadNodeID string) error {
	_ = ctx
	_ = deadNodeID
	return nil
}
