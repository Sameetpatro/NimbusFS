package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// DiskStore manages chunk files on local disk.
// one file per chunk, named by chunkID — avoids complex block management
type DiskStore struct {
	baseDir string
	mu      sync.RWMutex // protects concurrent reads and writes to same chunk
}

// NewDiskStore ensures the storage directory exists before serving chunks.
func NewDiskStore(baseDir string) (*DiskStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("storage.NewDiskStore: mkdir %s: %w", baseDir, err)
	}
	return &DiskStore{baseDir: baseDir}, nil
}

func (s *DiskStore) chunkPath(chunkID string) string {
	return filepath.Join(s.baseDir, chunkID)
}

// StoreChunk writes data to disk atomically.
// write to a .tmp file first, then rename — rename is atomic on linux,
// so a crash mid-write never leaves a corrupt chunk file
func (s *DiskStore) StoreChunk(chunk *domain.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	finalPath := s.chunkPath(chunk.ChunkID)
	tmpPath := finalPath + ".tmp"

	if err := os.WriteFile(tmpPath, chunk.Data, 0o600); err != nil {
		return fmt.Errorf("storage.StoreChunk: write tmp %s: %w", chunk.ChunkID, err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("storage.StoreChunk: rename %s: %w", chunk.ChunkID, err)
	}
	return nil
}

// RetrieveChunk reads chunk from disk and verifies checksum.
// checksum mismatch means bit rot or corruption — return error so caller tries another replica
func (s *DiskStore) RetrieveChunk(chunkID string) (*domain.Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.chunkPath(chunkID))
	if err != nil {
		return nil, fmt.Errorf("storage.RetrieveChunk: read %s: %w", chunkID, err)
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	chunk := &domain.Chunk{
		ChunkID:  chunkID,
		Data:     data,
		Checksum: checksum,
	}
	if checksum != chunkID {
		// chunk id is content hash; mismatch means on-disk bytes don't match expected identity
		return nil, fmt.Errorf("storage.RetrieveChunk: id/checksum mismatch for %s", chunkID)
	}
	return chunk, nil
}

// RetrieveChunkVerified loads a chunk and validates against an expected checksum field.
func (s *DiskStore) RetrieveChunkVerified(chunkID, expectedChecksum string) (*domain.Chunk, error) {
	chunk, err := s.RetrieveChunk(chunkID)
	if err != nil {
		return nil, err
	}
	if expectedChecksum != "" && chunk.Checksum != expectedChecksum {
		return nil, fmt.Errorf("storage.RetrieveChunkVerified: checksum mismatch for %s", chunkID)
	}
	if err := chunking.VerifyChecksum(chunk); err != nil {
		return nil, fmt.Errorf("storage.RetrieveChunkVerified: %w", err)
	}
	return chunk, nil
}

// DeleteChunk removes a chunk file from disk.
func (s *DiskStore) DeleteChunk(chunkID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.chunkPath(chunkID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage.DeleteChunk: %s: %w", chunkID, err)
	}
	return nil
}

// UsedBytes walks the chunk directory to report bytes used in heartbeats.
func (s *DiskStore) UsedBytes() (int64, error) {
	var total int64
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".tmp" {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// TotalBytes reports filesystem capacity via statfs for placement decisions on master.
func (s *DiskStore) TotalBytes() (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.baseDir, &stat); err != nil {
		return 0, fmt.Errorf("storage.TotalBytes: statfs: %w", err)
	}
	return int64(stat.Blocks) * int64(stat.Bsize), nil
}

// ChunkCount returns the number of chunk files stored locally.
func (s *DiskStore) ChunkCount() (int, error) {
	count := 0
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".tmp" {
			count++
		}
		return nil
	})
	return count, err
}
