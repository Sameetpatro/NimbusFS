package chunking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
)

// Chunker splits an io.Reader into domain.Chunk values.
// taking an io.Reader means we can chunk from http request body, file on disk, or test byte slices
type Chunker struct {
	chunkSize int64 // from config, default 4mb
}

// New creates a Chunker with the configured byte size per chunk.
func New(chunkSizeBytes int64) *Chunker {
	return &Chunker{chunkSize: chunkSizeBytes}
}

// Chunk reads from r, yielding chunks via the returned channel.
// channel-based streaming means caller processes chunks as they arrive, not after full split
func (c *Chunker) Chunk(ctx context.Context, fileID string, r io.Reader) (<-chan *domain.Chunk, <-chan error) {
	out := make(chan *domain.Chunk, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		buf := make([]byte, c.chunkSize)
		index := 0

		for {
			if err := ctx.Err(); err != nil {
				errCh <- fmt.Errorf("chunking.Chunk: %w", err)
				return
			}

			n, err := io.ReadFull(r, buf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if n == 0 {
					return
				}
			} else if err != nil {
				errCh <- fmt.Errorf("chunking.Chunk: read chunk %d: %w", index, err)
				return
			}

			data := make([]byte, n)
			copy(data, buf[:n])

			sum := sha256.Sum256(data)
			chunkID := hex.EncodeToString(sum[:])

			select {
			case out <- &domain.Chunk{
				ChunkID:  chunkID,
				FileID:   fileID,
				Index:    index,
				Data:     data,
				Checksum: chunkID,
			}:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}

			index++
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return
			}
		}
	}()

	return out, errCh
}

// Split reads from r until EOF and returns ordered chunks plus total file size.
func (c *Chunker) Split(fileID string, r io.Reader) ([]domain.Chunk, int64, error) {
	ctx := context.Background()
	chunksCh, errCh := c.Chunk(ctx, fileID, r)

	var chunks []domain.Chunk
	var total int64
	for chunk := range chunksCh {
		chunks = append(chunks, *chunk)
		total += int64(len(chunk.Data))
	}
	if err := <-errCh; err != nil {
		return nil, 0, err
	}
	return chunks, total, nil
}

// VerifyChecksum recomputes sha256 and compares to chunk.Checksum for read-path integrity.
func VerifyChecksum(chunk *domain.Chunk) error {
	sum := sha256.Sum256(chunk.Data)
	got := hex.EncodeToString(sum[:])
	if got != chunk.Checksum {
		return fmt.Errorf("checksum mismatch for chunk %s: want %s got %s", chunk.ChunkID, chunk.Checksum, got)
	}
	return nil
}

// VerifyBytesChecksum validates raw bytes against an expected sha256 hex digest.
func VerifyBytesChecksum(data []byte, expected string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != expected {
		return fmt.Errorf("checksum mismatch: want %s got %s", expected, got)
	}
	return nil
}
