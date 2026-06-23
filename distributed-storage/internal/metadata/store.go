package metadata

import (
	"context"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// Store is the metadata persistence contract master uses for files and nodes.
// interface here lets us swap boltdb for etcd later without rewriting handlers
type Store interface {
	// PutFile upserts file metadata after upload or replication state change
	PutFile(ctx context.Context, meta domain.FileMetadata) error
	// GetFile loads metadata by id for download and admin apis
	GetFile(ctx context.Context, fileID string) (*domain.FileMetadata, error)
	// DeleteFile removes metadata when the file is deleted from the cluster
	DeleteFile(ctx context.Context, fileID string) error
	// ListFiles returns all known files for listing endpoints
	ListFiles(ctx context.Context) ([]domain.FileMetadata, error)

	// PutNode registers or updates a storage node's view on the master
	PutNode(ctx context.Context, node domain.StorageNode) error
	// GetNode fetches a single node by stable id
	GetNode(ctx context.Context, nodeID string) (*domain.StorageNode, error)
	// ListNodes returns all registered nodes for placement decisions
	ListNodes(ctx context.Context) ([]domain.StorageNode, error)
	// DeleteNode removes a node record after it's been fully drained
	DeleteNode(ctx context.Context, nodeID string) error

	// Close releases underlying resources like boltdb file handles
	Close() error
}
