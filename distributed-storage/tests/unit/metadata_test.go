package unit_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
)

func TestBoltStore_SaveAndUpdateChunkNodes(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.NewBoltStore(dir)
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	meta := &domain.FileMetadata{
		FileID:    "file-1",
		FileName:  "test.txt",
		Size:      100,
		ChunkSize: 50,
		Chunks: []domain.ChunkInfo{
			{ChunkID: "chunk-a", Index: 0, Size: 50, NodeIDs: []string{"n1", "n2", "dead"}},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.SaveFile(ctx, meta); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	got, err := store.GetFile(ctx, "file-1")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if got.FileName != "test.txt" {
		t.Errorf("filename: got %q", got.FileName)
	}

	if err := store.UpdateChunkNodes(ctx, "file-1", "chunk-a", []string{"n1", "n2", "n3"}); err != nil {
		t.Fatalf("UpdateChunkNodes: %v", err)
	}

	got, _ = store.GetFile(ctx, "file-1")
	if len(got.Chunks[0].NodeIDs) != 3 || got.Chunks[0].NodeIDs[2] != "n3" {
		t.Errorf("updated nodes: %#v", got.Chunks[0].NodeIDs)
	}

	list, err := store.ListFiles(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListFiles: %v len=%d", err, len(list))
	}

	_ = filepath.Join(dir, "metadata.db")
}
