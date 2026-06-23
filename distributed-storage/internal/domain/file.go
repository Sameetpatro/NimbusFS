package domain

import "time"

// FileMetadata represents the master's knowledge of a stored file.
// using a struct here instead of a db row directly keeps domain logic clean
type FileMetadata struct {
	// uuid v4, generated at upload time so clients never collide on ids
	FileID string
	// original filename from client, preserved for download Content-Disposition headers
	FileName string
	// total bytes across all chunks, lets us validate reconstruction without scanning disk
	Size int64
	// size of each chunk in bytes, last chunk may be smaller when file isn't evenly divisible
	ChunkSize int64
	// ordered list of chunks that make up this file, index field on each ChunkInfo enforces order
	Chunks []ChunkInfo
	// wall clock time of first successful upload, used for listing and retention policies
	CreatedAt time.Time
	// updated when replication state changes so we know metadata is stale after node failures
	UpdatedAt time.Time
}
