import { useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useStore } from '../../store/useStore';
import { formatBytes } from '../../lib/chunking';

const CONTAINER_COLORS: Record<string, string> = {
  'dfs-master': '#6c5ce7',
  'dfs-storage-1': '#00b894',
  'dfs-storage-2': '#00cec9',
  'dfs-storage-3': '#0984e3',
  'dfs-storage-4': '#6c5ce7',
  'dfs-storage-5': '#e17055',
};

export function LogPanel() {
  const serverOnline = useStore((s) => s.serverOnline);
  const logs = useStore((s) => s.logs);
  const events = useStore((s) => s.events);
  const isSimulating = useStore((s) => s.isSimulating);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = 0;
  }, [logs.length]);

  return (
    <div className="log-panel glass side-panel">
      <div className="panel-header">
        <h3>{serverOnline ? 'Server Logs (Docker)' : 'Simulated Activity'}</h3>
        {serverOnline && (
          <span className="live-indicator on">STREAMING</span>
        )}
      </div>
      <p className="panel-explainer">
        {serverOnline
          ? 'Raw logs from your running master and storage containers.'
          : 'Plain-English trace of what the system would do — no cluster required.'}
      </p>
      <div className="logs-scroll" ref={scrollRef}>
        {serverOnline ? (
          <>
            <AnimatePresence initial={false}>
              {logs.map((log) => (
                <motion.div
                  key={log.id}
                  className="log-line"
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                  style={{ borderLeftColor: CONTAINER_COLORS[log.container] ?? '#636e72' }}
                >
                  <span className="log-container">{shortContainer(log.container)}</span>
                  <span className="log-text mono">{log.line}</span>
                </motion.div>
              ))}
            </AnimatePresence>
            {logs.length === 0 && (
              <p className="empty">Waiting for Docker log stream…</p>
            )}
          </>
        ) : (
          <DemoActivity events={events} isSimulating={isSimulating} />
        )}
      </div>
    </div>
  );
}

function DemoActivity({
  events,
  isSimulating,
}: {
  events: { id: string; type: string; message: string }[];
  isSimulating: boolean;
}) {
  if (!isSimulating && events.length === 0) {
    return <p className="empty">Drag the sample file to see simulated server activity</p>;
  }
  return (
    <>
      {events.map((ev) => (
        <div key={ev.id} className="log-line demo">
          <span className="log-container">{ev.type}</span>
          <span className="log-text">{ev.message}</span>
        </div>
      ))}
    </>
  );
}

function shortContainer(c: string): string {
  return c.replace('dfs-', '').replace('storage-', 's');
}

export function MetricsPanel() {
  const serverOnline = useStore((s) => s.serverOnline);
  const metrics = useStore((s) => s.metrics);
  const phase = useStore((s) => s.phase);
  const chunks = useStore((s) => s.chunks);

  return (
    <div className="metrics-panel glass side-panel">
      <div className="panel-header">
        <h3>Cluster Metrics</h3>
        <span className="badge">Prometheus + Grafana</span>
      </div>

      {serverOnline ? (
        <>
          <p className="panel-explainer">
            Live numbers from Prometheus — uploads, node health, disk usage across the cluster.
          </p>
          <div className="metrics-grid">
            {metrics.map((m) => (
              <MetricCard key={m.name} metric={m} />
            ))}
            {metrics.length === 0 && (
              <p className="empty">Polling Prometheus at :9099…</p>
            )}
          </div>
          <div className="grafana-embed">
            <iframe
              title="Grafana Dashboard"
              src="http://localhost:3000/d-solo/nimbusfs-dashboard/nimbusfs-dashboard?orgId=1&panelId=1&refresh=5s&theme=dark"
              loading="lazy"
            />
            <a
              href="http://localhost:3000/d/nimbusfs-dashboard/nimbusfs-dashboard"
              target="_blank"
              rel="noreferrer"
              className="grafana-link"
            >
              Open full Grafana dashboard →
            </a>
          </div>
        </>
      ) : (
        <div className="metrics-demo">
          <p className="panel-explainer">
            Start the cluster to see real Prometheus graphs. During demo, use the insights panel for per-file stats.
          </p>
          {phase !== 'idle' && chunks.length > 0 && (
            <div className="demo-chart-card">
              <span className="chart-header">This upload (simulated)</span>
              <div className="mini-bars">
                {chunks.map((c) => (
                  <div key={c.index} className="mini-bar-group">
                    <span>C{c.index}</span>
                    <div className="mini-bar-track">
                      <motion.div
                        className="mini-bar-fill"
                        initial={{ width: 0 }}
                        animate={{ width: '100%' }}
                        transition={{ delay: c.index * 0.2, duration: 0.6 }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
          <div className="metrics-grid demo-metrics">
            <DemoMetric label="dfs_upload_total" value="demo" />
            <DemoMetric label="dfs_node_health" value="5 alive" />
            <DemoMetric label="dfs_replication_lag" value="0" />
            <DemoMetric label="dfs_storage_used_bytes" value="simulated" />
          </div>
        </div>
      )}
    </div>
  );
}

function MetricCard({ metric }: { metric: { name: string; value: number; series?: { label: string; value: number }[] } }) {
  if (metric.series && metric.series.length > 0) {
    const max = Math.max(...metric.series.map((s) => s.value), 1);
    return (
      <div className="metric-card">
        <span className="metric-name">{metric.name}</span>
        <div className="bar-chart">
          {metric.series.map((s) => (
            <div key={s.label} className="bar-row">
              <span>{s.label}</span>
              <div className="bar-track">
                <motion.div
                  className="bar-fill"
                  initial={{ width: 0 }}
                  animate={{ width: `${(s.value / max) * 100}%` }}
                />
              </div>
              <span className="bar-val">{formatBytes(s.value)}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="metric-card">
      <span className="metric-name">{metric.name}</span>
      <motion.span
        className="metric-value"
        key={metric.value}
        initial={{ scale: 1.3, color: '#00f5ff' }}
        animate={{ scale: 1, color: '#dfe6e9' }}
      >
        {metric.name.includes('Bytes') ? formatBytes(metric.value) : metric.value.toLocaleString()}
      </motion.span>
    </div>
  );
}

function DemoMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric-card dim">
      <span className="metric-name">{label}</span>
      <span className="metric-value">{value}</span>
    </div>
  );
}
