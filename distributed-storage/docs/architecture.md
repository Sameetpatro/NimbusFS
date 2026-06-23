# Architecture

## System Overview

```
                         +------------------+
                         |     Client       |
                         |  (dfs CLI / curl)|
                         +--------+---------+
                                  | HTTP :8080
                                  v
+------------------+    gRPC :9090    +---------------------------+
|   Prometheus     |<--- scrape -------|         Master            |
|     :9090        |                   |  REST API + BoltDB meta   |
+--------+---------+                   |  Node registry + placement|
         |                             +----+------+------+--------+
         | metrics                            |      |      |
         v                                    | gRPC | gRPC | gRPC
+------------------+                          v      v      v
|    Grafana       |                    +-----+ +-----+ +-----+ ... (x5)
|     :3000        |                    | S-1 | | S-2 | | S-3 |
+------------------+                    +-----+ +-----+ +-----+
                                              \    |    /
                                               chunk replicas (RF=3)
```

Eight Docker services: **master**, **storage-1..5**, **prometheus**, **grafana**.

## Upload Data Flow

1. Client sends `POST /api/v1/upload` (multipart file) with API key or JWT.
2. Master assigns a `file_id` and streams the body through the chunker (default 4 MiB chunks).
3. For each chunk, the replication manager selects `replication_factor` alive nodes sorted by free space.
4. Master opens gRPC client streams to each selected storage node and sends chunk bytes in parallel (`errgroup`).
5. Storage nodes persist chunks atomically (write temp file, rename) and return success.
6. Master records `FileMetadata` (chunk IDs, node placements, checksums) in BoltDB.
7. Client receives `201` with `file_id`, size, and chunk count.

## Download Data Flow

1. Client sends `GET /api/v1/files/{fileId}/download` with auth.
2. Master loads `FileMetadata` from BoltDB; returns `404` if missing.
3. For each chunk in order, master calls `RetrieveChunk` on replica nodes with fallback.
4. Checksum is verified per chunk; corrupt replica triggers next node in `NodeIDs`.
5. Chunk bytes are streamed to the HTTP response (memory-bounded via `io.Copy`).
6. Client receives the reassembled file as `application/octet-stream`.

## Node Failure Recovery

1. Storage nodes send periodic gRPC heartbeats to master (default every 5s).
2. Heartbeat monitor scans registry; nodes silent longer than `dead_threshold` (15s) are marked **dead**.
3. Dead node ID is sent on `deadCh`; replicator runs `ReReplicateFromDeadNode` once per death event.
4. Replicator scans all file metadata for chunks referencing the dead node.
5. For each affected chunk, a healthy replica is read and copied to a new node via `ReplicateChunk` gRPC.
6. Metadata `NodeIDs` list is updated in BoltDB.
7. Downloads continue from surviving replicas during re-replication.

## Consistency Model

See [consistency.md](consistency.md). Summary: **strong metadata consistency** (single BoltDB writer), **eventual chunk replication** (ack after RF successes, background heal).

## Technology Choices

| Choice | Reason |
|--------|--------|
| **BoltDB (bbolt)** | Embedded, zero external DB for v1; serialized writes give strong metadata consistency; crash-safe. SQLite considered but bbolt fits key-value metadata access patterns. |
| **Gin** | Mature HTTP router with middleware ecosystem; faster to ship auth, rate limits, and multipart uploads than raw `net/http` alone. |
| **gRPC + protobuf** | Streaming chunk transfer between nodes; typed contracts in `proto/`; `WithTransportCredentials` for TLS. |
| **Prometheus + Grafana** | Standard metrics stack; custom `dfs_*` counters/histograms for uploads, node health, replication lag. |
| **ECDSA P-256 TLS** | Self-signed dev certs via `internal/tlsconfig`; shorter keys and faster handshakes than RSA at same security level. |
| **errgroup** | Parallel replica writes with first-error cancellation. |
| **sync.RWMutex** | Read-heavy node registry; heartbeats take write lock briefly. |

## Package Layout

```
cmd/master/          REST + gRPC master process
cmd/storage/         Storage node process
cmd/client/          dfs CLI (cobra)
internal/api/        REST handlers and upload/download pipeline
internal/metadata/   BoltDB MetadataStore
internal/replication/ Placement, fetch fallback, re-replication
internal/heartbeat/  Monitor + sender
internal/grpcserver/ Master/storage gRPC + client pool
internal/chunking/   Split, stream, checksum
internal/tlsconfig/  Load-or-generate TLS for dev/docker
```

## Graceful Shutdown

All long-running binaries trap `SIGINT`/`SIGTERM`, cancel background work, then shut down HTTP → gRPC → databases within a 30s timeout. Abrupt kills can drop in-flight requests; BoltDB is crash-safe but orderly close is preferred.
