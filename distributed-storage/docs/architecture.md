# NimbusFS Architecture (Phase 1)

Distributed file storage inspired by GFS, HDFS, and MinIO. Phase 1 delivers the scaffold; RPC handlers and upload/download paths land in later phases.

## High-Level Diagram

```
                         +------------------+
                         |   CLI Client     |
                         |  (cmd/client)    |
                         +--------+---------+
                                  | HTTP (Phase 2+)
                                  v
+------------------+    REST     +------------------+
|  Storage Node 1  |<---gRPC---->|                  |
+------------------+            |      Master      |
+------------------+            |   (cmd/master)   |
|  Storage Node 2  |<---gRPC--->|                  |
+------------------+            |  - REST API      |
+------------------+            |  - gRPC (nodes)  |
|  Storage Node N  |<---gRPC--->|  - BoltDB meta   |
+------------------+            +--------+---------+
                                         |
                    heartbeats /         | metadata
                    chunk reports        v
                                  +--------------+
                                  |  BoltDB      |
                                  |  (files,     |
                                  |   nodes)     |
                                  +--------------+

        Storage <-------- replicate --------> Storage
              (node-to-node StorageService gRPC, Phase 2+)

        +-------------+       scrape        +-------------+
        | Prometheus  |<---------------------| all nodes   |
        +------+------+                       +-------------+
               |
               v
        +-------------+
        |  Grafana    |
        +-------------+
```

## Components

| Component | Package / Path | Responsibility |
|-----------|----------------|----------------|
| Master | `cmd/master` | File metadata, node registry, REST API, placement decisions |
| Storage node | `cmd/storage` | Local chunk store, gRPC data plane, heartbeats |
| Client | `cmd/client` | Upload, download, list (Phase 2+) |
| Domain | `internal/domain` | Pure types: `FileMetadata`, `Chunk`, `StorageNode` |
| Config | `internal/config` | YAML + env overlay |
| Metadata | `internal/metadata` | `Store` interface + BoltDB implementation |
| Chunking | `internal/chunking` | Split files into fixed-size chunks with SHA-256 IDs |
| Replication | `internal/replication` | Replica selection and re-replication (stub in Phase 1) |
| Heartbeat | `internal/heartbeat` | Liveness sender (storage) and monitor (master) |
| Local storage | `internal/storage` | Disk-backed chunk read/write |
| REST API | `internal/api` | Gin handlers |
| gRPC | `internal/grpcserver` + `proto/` | Service definitions and server wrappers |

## Data Flow (Target — Phase 2+)

1. **Upload**: Client → Master REST → chunker splits file → master selects N nodes per chunk → parallel `StoreChunk` streams to storage nodes → nodes `ReportChunkStored` → master persists `FileMetadata`.
2. **Download**: Client → Master REST → master returns chunk map → client fetches chunks from any live replica via `RetrieveChunk` → reassembles file in order by `Index`.
3. **Failure**: Heartbeat monitor marks node `dead` → replication manager re-copies affected chunks to healthy nodes → master updates `ChunkInfo.NodeIDs`.

## Configuration

- Default: `configs/config.yaml`
- Env overrides: `NODE_ID`, `MASTER_*`, `STORAGE_*`, `LOG_LEVEL`, `METRICS_PORT`, etc.
- Docker: `deployments/docker-compose.yml` — 1 master, 5 storage nodes, Prometheus, Grafana, optional client profile.

## Network (Docker Compose)

All services on bridge network `dfs-net`. Master exposes `8080` (REST) and `9090` (gRPC). Storage nodes expose host ports `9091–9095` (gRPC) with per-node metrics on `9101–9105`.

## Phase Roadmap

| Phase | Focus |
|-------|--------|
| **1** (current) | Scaffold, config, logging, domain, protos, Docker skeleton |
| **2** | gRPC service implementations, upload/download, node registration |
| **3** | Auth (JWT/API key), TLS, prometheus metrics, re-replication |
| **4** | Integration tests, chaos/failure testing, production hardening |
