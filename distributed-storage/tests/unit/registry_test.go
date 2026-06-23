package unit_test

import (
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
)

func TestRegistryLifecycle(t *testing.T) {
	reg := registry.New()
	reg.Register(&domain.StorageNode{NodeID: "a", Address: "10.0.0.1:9091", Status: domain.NodeStatusSuspect, TotalSpace: 100})
	reg.UpdateHeartbeat("a", 10, 100, 1)

	node, ok := reg.Get("a")
	if !ok || node.Status != domain.NodeStatusAlive {
		t.Fatalf("heartbeat revive: %#v", node)
	}

	reg.MarkSuspect("a")
	node, _ = reg.Get("a")
	if node.Status != domain.NodeStatusSuspect {
		t.Fatalf("suspect: %s", node.Status)
	}

	if !reg.MarkDead("a") {
		t.Fatal("mark dead")
	}
	if reg.MarkDead("a") {
		t.Fatal("second mark dead should be false")
	}

	alive := reg.ListAlive()
	if len(alive) != 0 {
		t.Fatalf("expected 0 alive got %d", len(alive))
	}

	addr, ok := reg.GetAddress("a")
	if !ok || addr == "" {
		t.Fatal("address missing")
	}

	reg.Register(&domain.StorageNode{
		NodeID: "a", Address: "10.0.0.1:9091", Status: domain.NodeStatusAlive,
		LastHeartbeat: time.Now().Add(-time.Hour),
	})
	node, _ = reg.Get("a")
	if node.LastHeartbeat.Before(time.Now().Add(-30 * time.Minute)) {
		t.Fatal("register should preserve newer heartbeat from existing")
	}
}
