package observability

import (
	"strconv"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all prometheus counters and gauges for this service.
type Metrics struct {
	UploadTotal      prometheus.Counter
	DownloadTotal    prometheus.Counter
	DeleteTotal      prometheus.Counter
	UploadBytes      prometheus.Counter
	DownloadBytes    prometheus.Counter
	NodeHealthGauge  *prometheus.GaugeVec
	StorageUsedBytes *prometheus.GaugeVec
	ReplicationLag   prometheus.Gauge
	RequestDuration  *prometheus.HistogramVec
}

// NewMetrics creates and registers all metrics with the default registry.
func NewMetrics() *Metrics {
	return NewMetricsWithRegisterer(prometheus.DefaultRegisterer)
}

// NewMetricsWithRegisterer allows tests to use an isolated prometheus registry.
func NewMetricsWithRegisterer(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		UploadTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "dfs_upload_total",
			Help: "Total number of file uploads",
		}),
		DownloadTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "dfs_download_total",
			Help: "Total number of file downloads",
		}),
		DeleteTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "dfs_delete_total",
			Help: "Total number of file deletions",
		}),
		UploadBytes: factory.NewCounter(prometheus.CounterOpts{
			Name: "dfs_upload_bytes_total",
			Help: "Total bytes uploaded",
		}),
		DownloadBytes: factory.NewCounter(prometheus.CounterOpts{
			Name: "dfs_download_bytes_total",
			Help: "Total bytes downloaded",
		}),
		NodeHealthGauge: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dfs_node_health",
			Help: "Node health status (1=alive, 0.5=suspect, 0=dead)",
		}, []string{"node_id", "status"}),
		StorageUsedBytes: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dfs_storage_used_bytes",
			Help: "Bytes used per storage node",
		}, []string{"node_id"}),
		ReplicationLag: factory.NewGauge(prometheus.GaugeOpts{
			Name: "dfs_replication_lag",
			Help: "Chunks waiting for re-replication",
		}),
		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dfs_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}
}

// MetricsMiddleware records request duration for every api route.
func MetricsMiddleware(m *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		m.RequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())
	}
}

// UpdateNodeMetrics refreshes node gauges from registry state.
func (m *Metrics) UpdateNodeMetrics(reg *registry.NodeRegistry) {
	for _, node := range reg.List() {
		statusVal := nodeStatusValue(node.Status)
		m.NodeHealthGauge.WithLabelValues(node.NodeID, node.Status.String()).Set(statusVal)
		m.StorageUsedBytes.WithLabelValues(node.NodeID).Set(float64(node.UsedSpace))
	}
}

// UpdateReplicationLag sets lag gauge from replication manager.
func (m *Metrics) UpdateReplicationLag(replMgr *replication.Manager) {
	m.ReplicationLag.Set(replMgr.ReplicationLag())
}

func nodeStatusValue(s domain.NodeStatus) float64 {
	switch s {
	case domain.NodeStatusAlive:
		return 1
	case domain.NodeStatusSuspect:
		return 0.5
	case domain.NodeStatusDead:
		return 0
	default:
		return 0
	}
}

func (m *Metrics) ObserveUpload(bytes int64) {
	m.UploadTotal.Inc()
	m.UploadBytes.Add(float64(bytes))
}

func (m *Metrics) ObserveDownload(bytes int64) {
	m.DownloadTotal.Inc()
	m.DownloadBytes.Add(float64(bytes))
}

func (m *Metrics) ObserveDelete() {
	m.DeleteTotal.Inc()
}

func FormatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}
