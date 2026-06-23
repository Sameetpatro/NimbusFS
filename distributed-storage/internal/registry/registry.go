package registry

import (
	"sync"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// NodeRegistry holds live node state and is the single source of truth for master.
// embedding sync.RWMutex directly instead of wrapping each method keeps the struct lean
type NodeRegistry struct {
	mu    sync.RWMutex
	nodes map[string]*domain.StorageNode // nodeID -> node
}

// New creates an empty in-memory node registry.
func New() *NodeRegistry {
	return &NodeRegistry{
		nodes: make(map[string]*domain.StorageNode),
	}
}

// Register upserts a node record; idempotent for crash-recovery re-registration.
func (r *NodeRegistry) Register(node *domain.StorageNode) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.nodes[node.NodeID]; ok {
		if !node.LastHeartbeat.IsZero() && node.LastHeartbeat.Before(existing.LastHeartbeat) {
			node.LastHeartbeat = existing.LastHeartbeat
		}
	}
	cp := *node
	r.nodes[node.NodeID] = &cp
}

// UpdateHeartbeat patches liveness and capacity fields reported by storage nodes.
func (r *NodeRegistry) UpdateHeartbeat(nodeID string, used, total int64, chunkCount int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		return false
	}
	node.LastHeartbeat = time.Now()
	node.UsedSpace = used
	node.TotalSpace = total
	node.ChunkCount = chunkCount
	node.Status = domain.NodeStatusAlive
	return true
}

// Get returns a copy of the node so callers can't mutate registry state without the lock.
func (r *NodeRegistry) Get(nodeID string) (*domain.StorageNode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		return nil, false
	}
	cp := *node
	return &cp, true
}

// List returns copies of all registered nodes for monitor scans and placement.
func (r *NodeRegistry) List() []*domain.StorageNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*domain.StorageNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		cp := *node
		out = append(out, &cp)
	}
	return out
}

// ListAlive returns nodes currently marked alive for chunk placement decisions.
func (r *NodeRegistry) ListAlive() []*domain.StorageNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*domain.StorageNode
	for _, node := range r.nodes {
		if node.Status == domain.NodeStatusAlive {
			cp := *node
			out = append(out, &cp)
		}
	}
	return out
}

// MarkSuspect transitions a node to suspect when heartbeats are late but not yet dead.
func (r *NodeRegistry) MarkSuspect(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, ok := r.nodes[nodeID]; ok && node.Status != domain.NodeStatusDead {
		node.Status = domain.NodeStatusSuspect
	}
}

// MarkDead transitions a node to dead; monitor fires re-replication after this.
func (r *NodeRegistry) MarkDead(nodeID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, ok := r.nodes[nodeID]
	if !ok || node.Status == domain.NodeStatusDead {
		return false
	}
	node.Status = domain.NodeStatusDead
	return true
}

// GetAddress resolves node id to grpc host:port for client pool dials.
func (r *NodeRegistry) GetAddress(nodeID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, ok := r.nodes[nodeID]
	if !ok {
		return "", false
	}
	return node.Address, true
}
