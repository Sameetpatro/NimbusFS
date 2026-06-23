package observability

import (
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RegisterStorageNodeCollector exposes per-node disk gauges on storage processes.
func RegisterStorageNodeCollector(nodeID string, store *storage.DiskStore) {
	usedGauge := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dfs_storage_used_bytes",
		Help: "Bytes used on this storage node",
	}, []string{"node_id"})

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		for range ticker.C {
			used, err := store.UsedBytes()
			if err == nil {
				usedGauge.WithLabelValues(nodeID).Set(float64(used))
			}
		}
	}()
}
