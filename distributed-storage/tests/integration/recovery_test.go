//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
)

func TestMasterMetadataRecovery(t *testing.T) {
	dir := t.TempDir()

	store1, err := metadata.NewBoltStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := &domain.FileMetadata{
		FileID:    "recover-me",
		FileName:  "recovery.bin",
		Size:      1024,
		ChunkSize: 512,
		Chunks: []domain.ChunkInfo{
			{ChunkID: "chunk-1", Index: 0, Size: 512, NodeIDs: []string{"n1", "n2", "n3"}},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store1.SaveFile(context.Background(), meta); err != nil {
		t.Fatal(err)
	}
	if err := store1.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := metadata.NewBoltStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	got, err := store2.GetFile(context.Background(), "recover-me")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "recovery.bin" || len(got.Chunks) != 1 {
		t.Fatalf("unexpected metadata: %#v", got)
	}

	cluster := startTestCluster(t, 3)
	fileID := cluster.upload(t, bytes.Repeat([]byte("R"), 4096), "live.bin")
	got2, err := cluster.store.GetFile(context.Background(), fileID)
	if err != nil || got2.Size != 4096 {
		t.Fatalf("live upload metadata missing after recovery path: %v", err)
	}
}
