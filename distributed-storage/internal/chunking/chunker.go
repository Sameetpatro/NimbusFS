package chunking

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/google/uuid"
)

// Chunker splits an io.Reader into fixed-size domain.Chunk values with checksums.
type Chunker struct {
	// chunkSize caps each read buffer so memory stays bounded for multi-gb uploads
	chunkSize int64
}

// New creates a Chunker with the configured byte size per chunk.
func New(chunkSizeBytes int64) *Chunker {
	return &Chunker{chunkSize: chunkSizeBytes}
}

// Split reads from r until EOF and returns ordered chunks plus total file size.
func (c *Chunker) Split(fileID string, r io.Reader) ([]domain.Chunk, int64, error) {
	if fileID == "" {
		// caller should pass master's file id; empty would break re-replication lookups later
		fileID = uuid.NewString()
	}

	var chunks []domain.Chunk
	var total int64
	index := 0

	// reusable buffer avoids allocating a new []byte per chunk on large files
	buf := make([]byte, c.chunkSize)

	for {
		// read up to chunkSize bytes; short read at EOF is expected for the tail chunk
		n, err := io.ReadFull(r, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if n == 0 {
				break
			}
			// partial buffer is the last chunk; fall through to build it
		} else if err != nil {
			return nil, 0, fmt.Errorf("read chunk %d: %w", index, err)
		}

		// copy data because buf is reused on the next iteration
		data := make([]byte, n)
		copy(data, buf[:n])

		sum := sha256.Sum256(data)
		chunkID := hex.EncodeToString(sum[:])

		chunks = append(chunks, domain.Chunk{
			ChunkID:  chunkID,
			FileID:   fileID,
			Index:    index,
			Data:     data,
			Checksum: chunkID,
		})

		total += int64(n)
		index++

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}

	return chunks, total, nil
}

// VerifyChecksum recomputes sha256 and compares to chunk.Checksum for read-path integrity.
func VerifyChecksum(chunk *domain.Chunk) error {
	sum := sha256.Sum256(chunk.Data)
	got := hex.EncodeToString(sum[:])
	if got != chunk.Checksum {
		// mismatch means bit rot or tampering; don't serve corrupt bytes to clients
		return fmt.Errorf("checksum mismatch for chunk %s: want %s got %s", chunk.ChunkID, chunk.Checksum, got)
	}
	return nil
}
