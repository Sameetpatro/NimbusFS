package metadata

import (
	"context"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// MetadataStore defines the file metadata contract; bolt is one implementation.
// interface here means we can swap to postgres later without changing master logic
type MetadataStore interface {
	SaveFile(ctx context.Context, meta *domain.FileMetadata) error
	GetFile(ctx context.Context, fileID string) (*domain.FileMetadata, error)
	DeleteFile(ctx context.Context, fileID string) error
	ListFiles(ctx context.Context) ([]*domain.FileMetadata, error)
	// UpdateChunkNodes updates which nodes hold a specific chunk after re-replication
	UpdateChunkNodes(ctx context.Context, fileID, chunkID string, nodeIDs []string) error
}
