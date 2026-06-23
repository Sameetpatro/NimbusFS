package heartbeat

import (
	"context"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
)

// Sender periodically pings the master so it knows this storage node is alive.
type Sender struct {
	// interval between heartbeats, pulled from config so docker can tune without rebuild
	interval time.Duration
	// log helps operators see missed sends before the master marks the node suspect
	log *logger.Logger
	// nodeID is included in every heartbeat so master can update the right registry row
	nodeID string
	// sendFn is the actual transport call; injected so tests don't need a live grpc server
	sendFn func(ctx context.Context, nodeID string, usedSpace, totalSpace int64, chunkCount int) error
}

// NewSender builds a heartbeat sender with the given interval and callback.
func NewSender(interval time.Duration, nodeID string, log *logger.Logger, sendFn func(ctx context.Context, nodeID string, usedSpace, totalSpace int64, chunkCount int) error) *Sender {
	return &Sender{
		interval: interval,
		nodeID:   nodeID,
		log:      log.WithComponent("heartbeat-sender"),
		sendFn:   sendFn,
	}
}

// Run blocks until ctx is cancelled, sending heartbeats on each tick.
func (s *Sender) Run(ctx context.Context, stats func() (used, total int64, chunks int)) error {
	// ticker is cleaner than time.Sleep loops because it coalesces missed ticks on slow sends
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// context propagation lets upstream cancels reach the grpc call, preventing goroutine leaks
			return ctx.Err()
		case <-ticker.C:
			used, total, chunks := stats()
			if err := s.sendFn(ctx, s.nodeID, used, total, chunks); err != nil {
				// log but don't exit; transient master outages shouldn't kill the storage process
				s.log.Warn("heartbeat send failed", "error", err)
			}
		}
	}
}

// Monitor watches registered nodes and transitions status when heartbeats go missing.
type Monitor struct {
	// deadThreshold is how long without heartbeat before we mark a node dead
	deadThreshold time.Duration
	// suspectThreshold is half of dead so we get a warning state before re-replication fires
	suspectThreshold time.Duration
	log              *logger.Logger
	// onDead is called when a node transitions to dead so replication can kick in
	onDead func(ctx context.Context, node domain.StorageNode)
}

// NewMonitor creates a monitor with thresholds derived from config seconds.
func NewMonitor(deadThresholdSec int, log *logger.Logger, onDead func(ctx context.Context, node domain.StorageNode)) *Monitor {
	dead := time.Duration(deadThresholdSec) * time.Second
	return &Monitor{
		deadThreshold:    dead,
		suspectThreshold: dead / 2,
		log:              log.WithComponent("heartbeat-monitor"),
		onDead:           onDead,
	}
}

// EvaluateNode updates node.Status based on time since LastHeartbeat.
func (m *Monitor) EvaluateNode(ctx context.Context, node *domain.StorageNode) {
	since := time.Since(node.LastHeartbeat)

	switch {
	case since <= m.suspectThreshold:
		node.Status = domain.NodeStatusAlive
	case since <= m.deadThreshold:
		// suspect state gives us a grace window before expensive re-replication work
		if node.Status != domain.NodeStatusSuspect {
			m.log.Warn("node suspect", "node_id", node.NodeID, "since", since)
		}
		node.Status = domain.NodeStatusSuspect
	default:
		if node.Status != domain.NodeStatusDead {
			m.log.Error("node dead", "node_id", node.NodeID, "since", since)
			node.Status = domain.NodeStatusDead
			if m.onDead != nil {
				m.onDead(ctx, *node)
			}
		}
	}
}
