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
	filesBucket = []byte("files")
	nodesBucket = []byte("nodes")
)

// BoltStore implements Store on top of embedded boltdb for single-master metadata.
type BoltStore struct {
	// db handle is shared; boltdb serializes writers so we don't need an extra mutex layer
	db *bolt.DB
}

// NewBoltStore opens or creates the metadata database under dataDir.
func NewBoltStore(dataDir string) (*BoltStore, error) {
	// mkdir all parents because fresh containers mount empty volumes without intermediate dirs
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create metadata data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "metadata.db")
	db, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open boltdb %s: %w", dbPath, err)
	}

	// create buckets up front so read paths don't need lazy-create branches
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(filesBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(nodesBucket); err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init boltdb buckets: %w", err)
	}

	return &BoltStore{db: db}, nil
}

// PutFile serializes FileMetadata as json and stores it keyed by FileID.
func (s *BoltStore) PutFile(ctx context.Context, meta domain.FileMetadata) error {
	// honor upstream cancellation before we grab a boltdb write lock
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal file metadata: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(filesBucket)
		return b.Put([]byte(meta.FileID), data)
	})
}

// GetFile loads and unmarshals a file record by id.
func (s *BoltStore) GetFile(ctx context.Context, fileID string) (*domain.FileMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var meta domain.FileMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(filesBucket)
		raw := b.Get([]byte(fileID))
		if raw == nil {
			return fmt.Errorf("file %s not found", fileID)
		}
		return json.Unmarshal(raw, &meta)
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// DeleteFile removes a file record from the files bucket.
func (s *BoltStore) DeleteFile(ctx context.Context, fileID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(filesBucket).Delete([]byte(fileID))
	})
}

// ListFiles scans the files bucket and unmarshals every value.
func (s *BoltStore) ListFiles(ctx context.Context) ([]domain.FileMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var out []domain.FileMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(filesBucket)
		return b.ForEach(func(k, v []byte) error {
			var meta domain.FileMetadata
			if err := json.Unmarshal(v, &meta); err != nil {
				// skip corrupt rows in phase 1; phase 3 will add repair tooling
				return nil
			}
			out = append(out, meta)
			return nil
		})
	})
	return out, err
}

// PutNode upserts a storage node record keyed by NodeID.
func (s *BoltStore) PutNode(ctx context.Context, node domain.StorageNode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).Put([]byte(node.NodeID), data)
	})
}

// GetNode loads a single storage node by id.
func (s *BoltStore) GetNode(ctx context.Context, nodeID string) (*domain.StorageNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var node domain.StorageNode
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(nodesBucket).Get([]byte(nodeID))
		if raw == nil {
			return fmt.Errorf("node %s not found", nodeID)
		}
		return json.Unmarshal(raw, &node)
	})
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// ListNodes returns all registered storage nodes.
func (s *BoltStore) ListNodes(ctx context.Context) ([]domain.StorageNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var out []domain.StorageNode
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).ForEach(func(k, v []byte) error {
			var node domain.StorageNode
			if err := json.Unmarshal(v, &node); err != nil {
				return nil
			}
			out = append(out, node)
			return nil
		})
	})
	return out, err
}

// DeleteNode removes a node record after drain completes.
func (s *BoltStore) DeleteNode(ctx context.Context, nodeID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(nodesBucket).Delete([]byte(nodeID))
	})
}

// Close shuts down boltdb so process exit doesn't leave a stale lock file.
func (s *BoltStore) Close() error {
	return s.db.Close()
}
