package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	bolt "go.etcd.io/bbolt"
)

// bolt bucket names as constants so typos fail at compile time not at runtime
var (
	filesBucket         = []byte("files")
	nodesBucket         = []byte("nodes")
	filenameIndexBucket = []byte("filename_index")
)

// BoltStore implements MetadataStore using bbolt.
// bbolt is chosen over sqlite because it's pure go (no cgo), embeds cleanly,
// and is fast enough for our metadata volumes without a full sql engine
type BoltStore struct {
	db *bolt.DB // single writer, many readers — matches our access pattern
}

// NewBoltStore opens or creates the metadata database under dataDir.
func NewBoltStore(dataDir string) (*BoltStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("metadata.NewBoltStore: create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "metadata.db")
	db, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("metadata.NewBoltStore: open %s: %w", dbPath, err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{filesBucket, nodesBucket, filenameIndexBucket} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("metadata.NewBoltStore: init buckets: %w", err)
	}

	return &BoltStore{db: db}, nil
}

// SaveFile serializes metadata and updates the filename index in one transaction.
func (s *BoltStore) SaveFile(ctx context.Context, meta *domain.FileMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("metadata.SaveFile: marshal %s: %w", meta.FileID, err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(filesBucket).Put([]byte(meta.FileID), data); err != nil {
			return err
		}
		// filename index supports lookup by original client name without scanning all files
		return tx.Bucket(filenameIndexBucket).Put([]byte(meta.FileName), []byte(meta.FileID))
	})
}

// GetFile loads file metadata by id.
func (s *BoltStore) GetFile(ctx context.Context, fileID string) (*domain.FileMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var meta domain.FileMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(filesBucket).Get([]byte(fileID))
		if raw == nil {
			return fmt.Errorf("file %s not found", fileID)
		}
		return json.Unmarshal(raw, &meta)
	})
	if err != nil {
		return nil, fmt.Errorf("metadata.GetFile: %s: %w", fileID, err)
	}
	return &meta, nil
}

// DeleteFile removes file metadata and its filename index entry.
func (s *BoltStore) DeleteFile(ctx context.Context, fileID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		files := tx.Bucket(filesBucket)
		raw := files.Get([]byte(fileID))
		if raw == nil {
			return fmt.Errorf("file %s not found", fileID)
		}
		var meta domain.FileMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Errorf("metadata.DeleteFile: unmarshal %s: %w", fileID, err)
		}
		if err := files.Delete([]byte(fileID)); err != nil {
			return err
		}
		return tx.Bucket(filenameIndexBucket).Delete([]byte(meta.FileName))
	})
}

// ListFiles returns all file metadata records for crash recovery and listing apis.
func (s *BoltStore) ListFiles(ctx context.Context) ([]*domain.FileMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var out []*domain.FileMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(filesBucket).ForEach(func(_, v []byte) error {
			var meta domain.FileMetadata
			if err := json.Unmarshal(v, &meta); err != nil {
				return nil
			}
			m := meta
			out = append(out, &m)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("metadata.ListFiles: %w", err)
	}
	return out, nil
}

// UpdateChunkNodes patches replica locations for one chunk inside a file record.
func (s *BoltStore) UpdateChunkNodes(ctx context.Context, fileID, chunkID string, nodeIDs []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		files := tx.Bucket(filesBucket)
		raw := files.Get([]byte(fileID))
		if raw == nil {
			return fmt.Errorf("file %s not found", fileID)
		}
		var meta domain.FileMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Errorf("metadata.UpdateChunkNodes: unmarshal %s: %w", fileID, err)
		}

		found := false
		for i := range meta.Chunks {
			if meta.Chunks[i].ChunkID == chunkID {
				meta.Chunks[i].NodeIDs = append([]string(nil), nodeIDs...)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("chunk %s not found in file %s", chunkID, fileID)
		}

		data, err := json.Marshal(&meta)
		if err != nil {
			return fmt.Errorf("metadata.UpdateChunkNodes: marshal %s: %w", fileID, err)
		}
		return files.Put([]byte(fileID), data)
	})
}

// SaveNode persists a storage node record for crash recovery on master restart.
func (s *BoltStore) SaveNode(ctx context.Context, node *domain.StorageNode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("metadata.SaveNode: marshal %s: %w", node.NodeID, err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).Put([]byte(node.NodeID), data)
	})
}

// ListNodes loads all persisted nodes so master can rebuild registry after crash.
func (s *BoltStore) ListNodes(ctx context.Context) ([]*domain.StorageNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var out []*domain.StorageNode
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).ForEach(func(_, v []byte) error {
			var node domain.StorageNode
			if err := json.Unmarshal(v, &node); err != nil {
				return nil
			}
			n := node
			out = append(out, &n)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("metadata.ListNodes: %w", err)
	}
	return out, nil
}

// Close releases the boltdb handle.
func (s *BoltStore) Close() error {
	return s.db.Close()
}
