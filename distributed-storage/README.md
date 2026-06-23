# NimbusFS — Distributed File Storage

[![CI](https://github.com/Sameetpatro/NimbusFS/actions/workflows/ci.yml/badge.svg)](https://github.com/Sameetpatro/NimbusFS/actions/workflows/ci.yml)

Production-grade distributed file storage in Go, inspired by GFS/HDFS. NimbusFS splits files into content-addressed chunks, replicates them across storage nodes, and self-heals when nodes fail.

## Quick Start

```bash
cd distributed-storage
make docker-up          # start master + 5 storage nodes + prometheus + grafana
./scripts/demo.sh       # end-to-end upload/download/failure demo
```

CLI:

```bash
make build
./bin/dfs login --key demo-key
./bin/dfs upload ./myfile.bin
./bin/dfs list
./bin/dfs status
```

## Architecture Summary

- **Master** — REST API, metadata (BoltDB), node registry, placement, re-replication
- **Storage nodes** — gRPC chunk store with streaming transfer
- **Client** — Cobra CLI (`dfs`) for upload/download/list/status
- **Observability** — Prometheus metrics + Grafana dashboard

See [docs/architecture.md](docs/architecture.md) for diagrams and data flows.

## Documentation

| Doc | Description |
|-----|-------------|
| [architecture.md](docs/architecture.md) | System design, flows, technology choices |
| [api.md](docs/api.md) | REST API reference with curl examples |
| [deployment.md](docs/deployment.md) | Docker, TLS, scaling, backup |
| [consistency.md](docs/consistency.md) | Consistency model and tradeoffs |
| [resume-bullets.md](docs/resume-bullets.md) | Resume-ready project bullets |

## Development

```bash
make proto
make build
make test-unit
make test-integration
make test-cover      # reports internal/ coverage summary
make vet
make lint
```

Load test (requires [k6](https://k6.io/)):

```bash
k6 run tests/load/script.js
```

## Configuration

Default config: `configs/config.yaml`. Override via environment variables (`MASTER_*`, `NODE_ID`, `TLS_ENABLED`, `AUTH_API_KEYS`, etc.).

API keys for REST: set `auth.api_keys` in config or use `dfs login --key <key>`.

## License

MIT
