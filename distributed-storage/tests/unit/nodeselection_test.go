package unit_test

import (
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
)

func seedNodes(reg *registry.NodeRegistry) {
	reg.Register(&domain.StorageNode{NodeID: "low", Status: domain.NodeStatusAlive, TotalSpace: 1000, UsedSpace: 900})
	reg.Register(&domain.StorageNode{NodeID: "high", Status: domain.NodeStatusAlive, TotalSpace: 10000, UsedSpace: 100})
	reg.Register(&domain.StorageNode{NodeID: "mid", Status: domain.NodeStatusAlive, TotalSpace: 5000, UsedSpace: 1000})
	reg.Register(&domain.StorageNode{NodeID: "dead", Status: domain.NodeStatusDead, TotalSpace: 99999, UsedSpace: 0})
}

func TestSelectNodesPrefersFreeSpace(t *testing.T) {
	reg := registry.New()
	seedNodes(reg)
	mgr := replication.NewManager(reg, nil, grpcserver.NewClientPool(logger.New("error"), false), 3, logger.New("error"))

	// single replica placement always picks the node with most free space
	selected, err := mgr.SelectNodesForChunk(1)
	if err != nil {
		t.Fatal(err)
	}
	if selected[0].NodeID != "high" {
		t.Errorf("expected highest free space node, got %s", selected[0].NodeID)
	}

	// when picking 2 of 3 alive nodes, only top-two by free space are candidates
	ids := make(map[string]bool)
	for range 50 {
		nodes, err := mgr.SelectNodesForChunk(2)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			ids[n.NodeID] = true
			if n.NodeID == "low" {
				t.Fatal("low-capacity node should not be in top-2 selection")
			}
		}
	}
	if !ids["high"] || !ids["mid"] {
		t.Fatalf("expected high and mid in selections, got %v", ids)
	}
}

func TestSelectNodesSkipsDead(t *testing.T) {
	reg := registry.New()
	seedNodes(reg)
	mgr := replication.NewManager(reg, nil, grpcserver.NewClientPool(logger.New("error"), false), 3, logger.New("error"))

	for _, n := range selectedIDs(t, mgr, 3) {
		if n == "dead" {
			t.Fatal("dead node selected")
		}
	}
}

func TestSelectNodesInsufficientNodes(t *testing.T) {
	reg := registry.New()
	reg.Register(&domain.StorageNode{NodeID: "a", Status: domain.NodeStatusAlive, TotalSpace: 100})
	reg.Register(&domain.StorageNode{NodeID: "b", Status: domain.NodeStatusAlive, TotalSpace: 100})
	mgr := replication.NewManager(reg, nil, grpcserver.NewClientPool(logger.New("error"), false), 3, logger.New("error"))

	_, err := mgr.SelectNodesForChunk(3)
	if err == nil {
		t.Fatal("expected error")
	}
}

func selectedIDs(t *testing.T, mgr *replication.Manager, n int) []string {
	t.Helper()
	nodes, err := mgr.SelectNodesForChunk(n)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.NodeID
	}
	return ids
}
