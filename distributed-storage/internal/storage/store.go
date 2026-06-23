package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// LocalStore persists chunk bytes as files on disk under dataDir.
type LocalStore struct {
	// root is the base directory; each chunk becomes root/<fileID>/<chunkID>
	root string
}

// NewLocalStore ensures the data directory exists before any writes.
func NewLocalStore(dataDir string) (*LocalStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage data dir: %w", err)
	}
	return &LocalStore{root: dataDir}, nil
}

// chunkPath builds a stable path so chunks from different files never collide on disk.
func (s *LocalStore) chunkPath(fileID, chunkID string) string {
	return filepath.Join(s.root, fileID, chunkID)
}

// Put writes chunk bytes to disk and creates parent dirs as needed.
func (s *LocalStore) Put(ctx context.Context, chunk domain.Chunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path := s.chunkPath(chunk.FileID, chunk.ChunkID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}

	// write atomically via temp file in phase 2; direct write is fine for scaffold
	if err := os.WriteFile(path, chunk.Data, 0o600); err != nil {
		return fmt.Errorf("write chunk %s: %w", chunk.ChunkID, err)
	}
	return nil
}

// Get reads a chunk from disk into memory for grpc streaming in later phases.
func (s *LocalStore) Get(ctx context.Context, fileID, chunkID string) (*domain.Chunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := s.chunkPath(fileID, chunkID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read chunk %s: %w", chunkID, err)
	}

	return &domain.Chunk{
		ChunkID: chunkID,
		FileID:  fileID,
		Data:    data,
	}, nil
}

// Delete removes a chunk file from disk when master confirms it's unreferenced.
func (s *LocalStore) Delete(ctx context.Context, fileID, chunkID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.chunkPath(fileID, chunkID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete chunk %s: %w", chunkID, err)
	}
	return nil
}

// UsedBytes walks the tree to report disk usage for heartbeat payloads.
func (s *LocalStore) UsedBytes() (int64, error) {
	var total int64
	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// ChunkCount returns the number of chunk files on disk for load reporting.
func (s *LocalStore) ChunkCount() (int, error) {
	count := 0
	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}
