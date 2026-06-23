# NimbusFS Visualizer

Interactive 3D visualization of the NimbusFS distributed upload pipeline.

## Quick start

```bash
# Terminal 1 — NimbusFS cluster (optional, for Live mode)
cd distributed-storage
make up

# Terminal 2 — Visualizer
cd nimbusfs-visualizer
npm install
npm run dev
```

Open **http://localhost:5173**

## Modes

| Mode | Server | Behavior |
|------|--------|----------|
| **Demo** | Off | Cinematic animation of chunking, SHA-256 hashing, 3× replication, BoltDB write |
| **Live** | On (`localhost:8080`) | Real file upload, Docker log stream, Prometheus metrics, Grafana embed |

## Ports

| Service | Port |
|---------|------|
| Visualizer (Vite) | 5173 |
| Visualizer proxy | 4000 |
| NimbusFS Master | 8080 |
| Prometheus | 9099 |
| Grafana | 3000 |

## Environment

```bash
NIMBUSFS_MASTER_URL=http://localhost:8080
NIMBUSFS_API_KEY=demo-key
PROMETHEUS_URL=http://localhost:9099
GRAFANA_URL=http://localhost:3000
```
