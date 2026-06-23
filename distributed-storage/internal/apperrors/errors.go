package apperrors

import "errors"

// ErrInsufficientStorage is returned when not enough healthy nodes exist for replication.
var ErrInsufficientStorage = errors.New("insufficient storage capacity")

// ErrFileNotFound means metadata has no record for the requested file id.
var ErrFileNotFound = errors.New("file not found")

// ErrChunkUnavailable means no replica could serve the chunk.
var ErrChunkUnavailable = errors.New("chunk unavailable from all replicas")
