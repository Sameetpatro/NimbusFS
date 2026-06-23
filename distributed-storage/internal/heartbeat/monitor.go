package heartbeat

import (
	"context"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
)

// Replicator triggers chunk recovery when nodes are marked dead.
// interface here avoids an import cycle with the replication package
type Replicator interface {
	ReReplicateFromDeadNode(ctx context.Context, deadNodeID string)
}

// Monitor runs as a long-lived goroutine, checking heartbeat timestamps.
// using a ticker instead of sleep because ticker accounts for processing time in the interval
type Monitor struct {
	registry     *registry.NodeRegistry
	store        metadata.MetadataStore
	replMgr      Replicator
	deadCh       chan string
	checkPeriod  time.Duration
	deadAfter    time.Duration
	suspectAfter time.Duration
	log          *logger.Logger
}

// NewMonitor wires dependencies for the background liveness scanner.
func NewMonitor(
	reg *registry.NodeRegistry,
	store metadata.MetadataStore,
	replMgr Replicator,
	deadCh chan string,
	checkPeriodSec, deadThresholdSec int,
	log *logger.Logger,
) *Monitor {
	deadAfter := time.Duration(deadThresholdSec) * time.Second
	return &Monitor{
		registry:     reg,
		store:        store,
		replMgr:      replMgr,
		deadCh:       deadCh,
		checkPeriod:  time.Duration(checkPeriodSec) * time.Second,
		deadAfter:    deadAfter,
		suspectAfter: deadAfter / 2,
		log:          log.WithComponent("heartbeat-monitor"),
	}
}

// Run starts the monitoring loop. call in a goroutine from main.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.checkPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scan(ctx)
		}
	}
}

func (m *Monitor) scan(ctx context.Context) {
	nodes := m.registry.List()
	now := time.Now()

	for _, node := range nodes {
		if node.LastHeartbeat.IsZero() {
			continue
		}

		since := now.Sub(node.LastHeartbeat)
		switch {
		case since > m.deadAfter:
			if m.registry.MarkDead(node.NodeID) {
				m.log.Error("node marked dead", "node_id", node.NodeID, "silent_for", since)
				select {
				case m.deadCh <- node.NodeID:
				default:
					go m.replMgr.ReReplicateFromDeadNode(ctx, node.NodeID)
				}
			}
		case since > m.suspectAfter:
			m.registry.MarkSuspect(node.NodeID)
		}
	}
}
