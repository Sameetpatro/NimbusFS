package domain

// ChunkInfo links a chunk to its replica locations.
// storing node ids as strings lets us survive node restarts with new ips
type ChunkInfo struct {
	// sha256 of chunk content for integrity checks without re-reading from disk on master
	ChunkID string
	// position in file, needed to reconstruct in order when chunks arrive out of band
	Index int
	// bytes in this specific chunk, last chunk is often smaller than ChunkSize
	Size int64
	// which storage nodes hold a replica of this chunk, master uses this for read routing
	NodeIDs []string
}

// Chunk is the unit of storage on a storage node.
// keeping data as []byte lets us stream directly from disk into grpc
type Chunk struct {
	// matches ChunkInfo.ChunkID on master so replication and reads agree on identity
	ChunkID string
	// back-reference to parent file, used during re-replication when we need file context
	FileID string
	// position in file, used to serve chunks in order during ranged reads
	Index int
	// raw bytes, max ChunkSize from config; slice not io.Reader so grpc can chunk the payload
	Data []byte
	// sha256 of Data, verified on read to detect bit rot before returning to client
	Checksum string
}
