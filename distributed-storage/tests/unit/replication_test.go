package unit_test

import (
	"context"
	"testing"
)

func TestReReplicationOnNodeDeath(t *testing.T) {
	cluster := startMiniCluster(t, 4)
	data := []byte("re-replication test")
	fileID := cluster.upload(t, data, "repl.bin")

	meta, err := cluster.store.GetFile(context.Background(), fileID)
	if err != nil {
		t.Fatal(err)
	}
	deadID := meta.Chunks[0].NodeIDs[0]
	cluster.registry.MarkDead(deadID)

	cluster.replMgr.ReReplicateFromDeadNode(context.Background(), deadID)

	meta, err = cluster.store.GetFile(context.Background(), fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Chunks[0].NodeIDs) != 3 {
		t.Fatalf("expected 3 replicas, got %v", meta.Chunks[0].NodeIDs)
	}
	for _, id := range meta.Chunks[0].NodeIDs {
		if id == deadID {
			t.Fatal("dead node still listed after re-replication")
		}
	}
}
