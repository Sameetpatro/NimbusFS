import { useMemo } from 'react';
import { motion } from 'framer-motion';
import { useStore } from '../../store/useStore';
import { formatBytes } from '../../lib/chunking';
import { STORAGE_NODES } from '../../lib/constants';

export function InsightsDashboard() {
  const phase = useStore((s) => s.phase);
  const chunks = useStore((s) => s.chunks);
  const activeChunk = useStore((s) => s.activeChunkIndex);
  const fileSize = useStore((s) => s.fileSize);
  const nodes = useStore((s) => s.nodes);
  const metrics = useStore((s) => s.metrics);
  const serverOnline = useStore((s) => s.serverOnline);
  const isSimulating = useStore((s) => s.isSimulating);

  const progress = useMemo(() => {
    const map: Record<string, number> = {
      idle: 0,
      uploading: 12,
      chunking: 35,
      replicating: 65,
      metadata: 88,
      complete: 100,
    };
    return map[phase] ?? 0;
  }, [phase]);

  const bytesProcessed = useMemo(() => {
    if (!fileSize || chunks.length === 0) return 0;
    if (phase === 'complete') return fileSize;
    if (activeChunk < 0) return 0;
    let sum = 0;
    for (let i = 0; i <= activeChunk && i < chunks.length; i++) {
      sum += chunks[i].size;
    }
    return sum;
  }, [fileSize, chunks, activeChunk, phase]);

  const replicaCount = useMemo(() => {
    const counts = new Map<string, number>();
    STORAGE_NODES.forEach((n) => counts.set(n.id, 0));
    const limit = phase === 'complete' ? chunks.length : activeChunk + 1;
    chunks.slice(0, Math.max(0, limit)).forEach((c) => {
      c.nodeIds.forEach((nid) => counts.set(nid, (counts.get(nid) ?? 0) + 1));
    });
    return counts;
  }, [chunks, activeChunk, phase]);

  const maxReplicas = Math.max(...replicaCount.values(), 1);

  const uploadMetric = metrics.find((m) => m.name === 'Uploads');
  const aliveMetric = metrics.find((m) => m.name === 'Alive Nodes');

  return (
    <div className="insights-dashboard glass">
      <h3>Live Insights</h3>

      <div className="insight-row">
        <div className="donut-wrap">
          <svg viewBox="0 0 36 36" className="donut">
            <circle className="donut-bg" cx="18" cy="18" r="15.9" fill="none" strokeWidth="3" />
            <motion.circle
              className="donut-fill"
              cx="18"
              cy="18"
              r="15.9"
              fill="none"
              strokeWidth="3"
              strokeLinecap="round"
              strokeDasharray="100 100"
              initial={{ strokeDashoffset: 100 }}
              animate={{ strokeDashoffset: 100 - progress }}
              transition={{ duration: 0.5 }}
            />
          </svg>
          <span className="donut-label">{progress}%</span>
        </div>
        <div className="insight-stats">
          <div className="stat">
            <span className="stat-val">{chunks.length || '—'}</span>
            <span className="stat-key">Chunks</span>
          </div>
          <div className="stat">
            <span className="stat-val">3×</span>
            <span className="stat-key">Replicas</span>
          </div>
          <div className="stat">
            <span className="stat-val">{nodes.filter((n) => n.status === 'alive').length}</span>
            <span className="stat-key">Nodes up</span>
          </div>
        </div>
      </div>

      {(isSimulating || phase === 'complete') && fileSize > 0 && (
        <div className="bytes-chart">
          <div className="chart-header">
            <span>Bytes processed</span>
            <span>{formatBytes(bytesProcessed)} / {formatBytes(fileSize)}</span>
          </div>
          <div className="chart-track">
            <motion.div
              className="chart-fill"
              animate={{ width: `${fileSize ? (bytesProcessed / fileSize) * 100 : 0}%` }}
            />
          </div>
        </div>
      )}

      {chunks.length > 0 && (
        <div className="replica-heatmap">
          <span className="chart-header">Chunks per node (this file)</span>
          {STORAGE_NODES.map((sn) => {
            const count = replicaCount.get(sn.id) ?? 0;
            return (
              <div key={sn.id} className="heat-row">
                <span>{sn.label}</span>
                <div className="heat-track">
                  <motion.div
                    className="heat-fill"
                    initial={{ width: 0 }}
                    animate={{ width: `${(count / maxReplicas) * 100}%` }}
                    transition={{ duration: 0.4 }}
                  />
                </div>
                <span className="heat-val">{count}</span>
              </div>
            );
          })}
        </div>
      )}

      {serverOnline && uploadMetric && (
        <div className="cluster-stats">
          <div className="cluster-stat">
            <span>Total uploads (cluster)</span>
            <strong>{uploadMetric.value.toLocaleString()}</strong>
          </div>
          {aliveMetric && (
            <div className="cluster-stat">
              <span>Alive nodes</span>
              <strong>{aliveMetric.value}</strong>
            </div>
          )}
        </div>
      )}

      {!isSimulating && phase === 'idle' && (
        <p className="insight-hint">
          Upload a file to see chunk distribution, byte progress, and replication heatmap update in real time.
        </p>
      )}
    </div>
  );
}
