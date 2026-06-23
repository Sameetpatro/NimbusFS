package unit_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
)

func TestChunker_SplitAndChecksum(t *testing.T) {
	const chunkSize = 16
	data := []byte("hello distributed storage chunk test data")
	chunker := chunking.New(chunkSize)

	chunks, total, err := chunker.Split("file-1", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if total != int64(len(data)) {
		t.Errorf("total size: got %d want %d", total, len(data))
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	for i, ch := range chunks {
		if ch.Index != i {
			t.Errorf("chunk %d index: got %d", i, ch.Index)
		}
		if err := chunking.VerifyChecksum(&ch); err != nil {
			t.Errorf("chunk %d checksum: %v", i, err)
		}
		if ch.ChunkID != ch.Checksum {
			t.Errorf("chunk %d: ChunkID should equal checksum hash", i)
		}
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	chunk := &domain.Chunk{
		ChunkID:  "abc",
		Data:     []byte("corrupt"),
		Checksum: "wrong",
	}
	if err := chunking.VerifyChecksum(chunk); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestManager_SelectNodesForChunk(t *testing.T) {
	registry := registry.New()
	registry.Register(&domain.StorageNode{
		NodeID: "n1", Address: "n1:9091", Status: domain.NodeStatusAlive,
		TotalSpace: 1000, UsedSpace: 100,
	})
	registry.Register(&domain.StorageNode{
		NodeID: "n2", Address: "n2:9091", Status: domain.NodeStatusAlive,
		TotalSpace: 2000, UsedSpace: 500,
	})
	registry.Register(&domain.StorageNode{
		NodeID: "n3", Address: "n3:9091", Status: domain.NodeStatusAlive,
		TotalSpace: 1500, UsedSpace: 200,
	})
	registry.Register(&domain.StorageNode{
		NodeID: "dead", Address: "dead:9091", Status: domain.NodeStatusDead,
		TotalSpace: 9999, UsedSpace: 0,
	})

	mgr := replication.NewManager(registry, nil, grpcserver.NewClientPool(logger.New("error")), 3, logger.New("error"))

	selected, err := mgr.SelectNodesForChunk(2)
	if err != nil {
		t.Fatalf("SelectNodesForChunk: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("got %d nodes want 2", len(selected))
	}

	ids := make(map[string]struct{})
	for _, n := range selected {
		ids[n.NodeID] = struct{}{}
		if n.Status != domain.NodeStatusAlive {
			t.Errorf("selected non-alive node %s", n.NodeID)
		}
	}
	if _, ok := ids["dead"]; ok {
		t.Error("dead node should not be selected")
	}
}

func TestManager_SelectNodesForChunk_NotEnough(t *testing.T) {
	registry := registry.New()
	registry.Register(&domain.StorageNode{NodeID: "only", Status: domain.NodeStatusAlive, TotalSpace: 100})

	mgr := replication.NewManager(registry, nil, grpcserver.NewClientPool(logger.New("error")), 3, logger.New("error"))
	_, err := mgr.SelectNodesForChunk(2)
	if err == nil {
		t.Fatal("expected error when not enough nodes")
	}
	if !strings.Contains(err.Error(), "need 2 nodes") {
		t.Errorf("unexpected error: %v", err)
	}
}
