package unit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
)

func collectChunks(t *testing.T, chunker *chunking.Chunker, fileID string, r io.Reader) []*domain.Chunk {
	t.Helper()
	ctx := context.Background()
	ch, errCh := chunker.Chunk(ctx, fileID, r)
	var out []*domain.Chunk
	for c := range ch {
		out = append(out, c)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("chunk: %v", err)
	}
	return out
}

func TestChunkerSplitExactMultiple(t *testing.T) {
	const size = 2048
	chunker := chunking.New(1024)
	data := bytes.Repeat([]byte("x"), size)
	chunks := collectChunks(t, chunker, "f1", bytes.NewReader(data))
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks got %d", len(chunks))
	}
}

func TestChunkerSplitWithRemainder(t *testing.T) {
	chunker := chunking.New(1024)
	data := bytes.Repeat([]byte("y"), 1024+512)
	chunks := collectChunks(t, chunker, "f1", bytes.NewReader(data))
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks got %d", len(chunks))
	}
	if len(chunks[1].Data) != 512 {
		t.Fatalf("last chunk size %d want 512", len(chunks[1].Data))
	}
}

func TestChunkerEmptyFile(t *testing.T) {
	chunker := chunking.New(1024)
	chunks := collectChunks(t, chunker, "f1", bytes.NewReader(nil))
	if len(chunks) != 0 {
		t.Fatalf("want 0 chunks got %d", len(chunks))
	}
}

func TestChunkerContextCancel(t *testing.T) {
	chunker := chunking.New(1024)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, errCh := chunker.Chunk(ctx, "f1", strings.NewReader("data"))
	for range ch {
	}
	if err := <-errCh; err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestChunkChecksumVerification(t *testing.T) {
	store, _ := storage.NewDiskStore(t.TempDir())
	chunks, _, _ := chunking.New(1024).Split("f1", bytes.NewReader([]byte("checksum-test")))
	chunk := &chunks[0]
	_ = store.StoreChunk(chunk)

	got, err := store.RetrieveChunk(chunk.ChunkID)
	if err != nil {
		t.Fatal(err)
	}
	got.Data[0] ^= 0xff
	if err := chunking.VerifyChecksum(got); err == nil {
		t.Fatal("expected checksum error on corrupted data")
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	chunk := &domain.Chunk{ChunkID: "abc", Data: []byte("corrupt"), Checksum: "wrong"}
	if err := chunking.VerifyChecksum(chunk); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestVerifyBytesChecksum(t *testing.T) {
	data := []byte("hello")
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])
	if err := chunking.VerifyBytesChecksum(data, expected); err != nil {
		t.Fatal(err)
	}
	if err := chunking.VerifyBytesChecksum(data, "bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestChunkerContextCancelMidStream(t *testing.T) {
	chunker := chunking.New(1024)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	stall := &stallAfterFirstRead{ctx: ctx}
	ch, errCh := chunker.Chunk(ctx, "f1", stall)
	go cancel()

	for range ch {
	}
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cancel")
	}
}

// stallAfterFirstRead returns one byte then blocks until context is cancelled.
type stallAfterFirstRead struct {
	ctx     context.Context
	started bool
}

func (s *stallAfterFirstRead) Read(p []byte) (int, error) {
	if !s.started {
		s.started = true
		p[0] = 'x'
		return 1, nil
	}
	<-s.ctx.Done()
	return 0, s.ctx.Err()
}
