package unit_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
)

func TestBoltStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := metadata.NewBoltStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	meta := &domain.FileMetadata{
		FileID: "f-1", FileName: "a.txt", Size: 10, ChunkSize: 4,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := store.SaveFile(context.Background(), meta); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetFile(context.Background(), "f-1")
	if err != nil || got.FileName != "a.txt" {
		t.Fatalf("get: %v %#v", err, got)
	}
}

func TestBoltStoreDeleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	store, _ := metadata.NewBoltStore(dir)
	defer func() { _ = store.Close() }()

	meta := &domain.FileMetadata{FileID: "f-del", FileName: "del.txt", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = store.SaveFile(context.Background(), meta)
	_ = store.DeleteFile(context.Background(), "f-del")

	_, err := store.GetFile(context.Background(), "f-del")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestBoltStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store, _ := metadata.NewBoltStore(dir)
	meta := &domain.FileMetadata{FileID: "persist", FileName: "p.bin", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = store.SaveFile(context.Background(), meta)
	_ = store.Close()

	store2, err := metadata.NewBoltStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store2.Close() }()

	got, err := store2.GetFile(context.Background(), "persist")
	if err != nil || got.FileID != "persist" {
		t.Fatalf("persistence failed: %v", err)
	}
	_ = filepath.Join(dir, "metadata.db")
}

func TestBoltStoreUpdateChunkNodes(t *testing.T) {
	dir := t.TempDir()
	store, _ := metadata.NewBoltStore(dir)
	defer func() { _ = store.Close() }()

	meta := &domain.FileMetadata{
		FileID: "f-1", FileName: "test.txt", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Chunks: []domain.ChunkInfo{{ChunkID: "c1", NodeIDs: []string{"n1", "n2"}}},
	}
	_ = store.SaveFile(context.Background(), meta)
	_ = store.UpdateChunkNodes(context.Background(), "f-1", "c1", []string{"n1", "n2", "n3"})

	got, _ := store.GetFile(context.Background(), "f-1")
	if len(got.Chunks[0].NodeIDs) != 3 {
		t.Fatalf("nodes: %v", got.Chunks[0].NodeIDs)
	}
}

func TestBoltStoreSaveNodeAndList(t *testing.T) {
	dir := t.TempDir()
	store, _ := metadata.NewBoltStore(dir)
	defer func() { _ = store.Close() }()

	node := &domain.StorageNode{NodeID: "n1", Address: "h:1", Status: domain.NodeStatusAlive}
	if err := store.SaveNode(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListNodes(context.Background())
	if err != nil || len(nodes) != 1 {
		t.Fatalf("list nodes: %v len=%d", err, len(nodes))
	}
}
