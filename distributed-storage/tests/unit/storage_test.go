package unit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
)

func TestDiskStore_AtomicWriteAndChecksum(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewDiskStore(dir)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}

	data := []byte("atomic chunk payload")
	chunks, _, err := chunking.New(1024).Split("f1", bytes.NewReader(data))
	if err != nil || len(chunks) != 1 {
		t.Fatalf("split: %v chunks=%d", err, len(chunks))
	}

	chunk := &chunks[0]
	if err := store.StoreChunk(chunk); err != nil {
		t.Fatalf("StoreChunk: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, chunk.ChunkID+".tmp")); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after successful store")
	}

	got, err := store.RetrieveChunk(chunk.ChunkID)
	if err != nil {
		t.Fatalf("RetrieveChunk: %v", err)
	}
	if string(got.Data) != string(data) {
		t.Errorf("data mismatch")
	}
	if err := chunking.VerifyChecksum(got); err != nil {
		t.Errorf("checksum: %v", err)
	}
}

func TestDiskStore_DeleteChunk(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.NewDiskStore(dir)
	chunks, _, _ := chunking.New(1024).Split("f1", bytes.NewReader([]byte("delete-me")))
	chunk := &chunks[0]
	_ = store.StoreChunk(chunk)
	if err := store.DeleteChunk(chunk.ChunkID); err != nil {
		t.Fatalf("DeleteChunk: %v", err)
	}
	if _, err := store.RetrieveChunk(chunk.ChunkID); err == nil {
		t.Fatal("expected error after delete")
	}
}
