package unit_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/heartbeat"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
)

type mockReplicator struct {
	calls atomic.Int32
}

func (m *mockReplicator) ReReplicateFromDeadNode(ctx context.Context, deadNodeID string) {
	m.calls.Add(1)
}

func TestMonitorMarksDead(t *testing.T) {
	reg := registry.New()
	reg.Register(&domain.StorageNode{
		NodeID: "n1", Status: domain.NodeStatusAlive,
		LastHeartbeat: time.Now().Add(-20 * time.Second),
	})

	deadCh := make(chan string, 1)
	mon := heartbeat.NewMonitor(reg, nil, &mockReplicator{}, deadCh, 1, 5, logger.New("error"))
	mon.ScanOnce(context.Background())

	node, _ := reg.Get("n1")
	if node.Status != domain.NodeStatusDead {
		t.Fatalf("expected dead got %s", node.Status)
	}
}

func TestMonitorRevivesNode(t *testing.T) {
	reg := registry.New()
	reg.Register(&domain.StorageNode{
		NodeID: "n1", Status: domain.NodeStatusDead,
		LastHeartbeat: time.Now().Add(-30 * time.Second),
	})

	reg.UpdateHeartbeat("n1", 10, 1000, 1)
	node, _ := reg.Get("n1")
	if node.Status != domain.NodeStatusAlive {
		t.Fatalf("expected alive after heartbeat got %s", node.Status)
	}
}

func TestMonitorFiresReReplication(t *testing.T) {
	reg := registry.New()
	reg.Register(&domain.StorageNode{
		NodeID: "n1", Status: domain.NodeStatusAlive,
		LastHeartbeat: time.Now().Add(-30 * time.Second),
	})

	repl := &mockReplicator{}
	deadCh := make(chan string, 1)
	ctx := context.Background()
	go func() {
		id := <-deadCh
		repl.ReReplicateFromDeadNode(ctx, id)
	}()

	mon := heartbeat.NewMonitor(reg, nil, repl, deadCh, 1, 10, logger.New("error"))
	mon.ScanOnce(ctx)

	time.Sleep(100 * time.Millisecond)
	if repl.calls.Load() != 1 {
		t.Fatalf("expected 1 re-replication call got %d", repl.calls.Load())
	}
}
