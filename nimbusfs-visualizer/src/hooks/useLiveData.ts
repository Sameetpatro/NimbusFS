import { useEffect, useCallback } from 'react';
import { useStore } from '../store/useStore';
import type { PrometheusMetric } from '../lib/constants';

const METRIC_QUERIES: { name: string; query: string; multi?: boolean }[] = [
  { name: 'Uploads', query: 'dfs_upload_total' },
  { name: 'Upload Bytes', query: 'dfs_upload_bytes_total' },
  { name: 'Downloads', query: 'dfs_download_total' },
  { name: 'Alive Nodes', query: 'count(dfs_node_health{status="alive"} == 1)' },
  { name: 'Replication Lag', query: 'dfs_replication_lag' },
  { name: 'Storage Used', query: 'dfs_storage_used_bytes', multi: true },
];

export function useServerHealth() {
  const setServerOnline = useStore((s) => s.setServerOnline);
  const setHealthChecked = useStore((s) => s.setHealthChecked);

  useEffect(() => {
    let first = true;

    const check = async () => {
      try {
        const res = await fetch('/api/health');
        const data = await res.json();
        setServerOnline(data.online === true);
      } catch {
        setServerOnline(false);
      } finally {
        if (first) {
          setHealthChecked(true);
          first = false;
        }
      }
    };

    check();
    const id = setInterval(check, 4000);
    return () => clearInterval(id);
  }, [setServerOnline, setHealthChecked]);
}

export function useClusterSync() {
  const serverOnline = useStore((s) => s.serverOnline);
  const setNodes = useStore((s) => s.setNodes);

  useEffect(() => {
    if (!serverOnline) return;

    const sync = async () => {
      try {
        const res = await fetch('/api/cluster/status');
        const data = await res.json();
        if (data.nodes) {
          setNodes(
            data.nodes.map(
              (n: { NodeID: string; Status: string; TotalSpace: number; UsedSpace: number }) => ({
                id: n.NodeID,
                label: n.NodeID.replace('storage-node-', 'Node '),
                status: n.Status as 'alive' | 'suspect' | 'dead',
                totalSpace: n.TotalSpace,
                usedSpace: n.UsedSpace,
              }),
            ),
          );
        }
      } catch {
        /* cluster offline */
      }
    };

    sync();
    const id = setInterval(sync, 5000);
    return () => clearInterval(id);
  }, [serverOnline, setNodes]);
}

export function usePrometheusMetrics() {
  const serverOnline = useStore((s) => s.serverOnline);
  const setMetrics = useStore((s) => s.setMetrics);

  useEffect(() => {
    if (!serverOnline) {
      setMetrics([]);
      return;
    }

    const poll = async () => {
      const metrics: PrometheusMetric[] = [];

      for (const { name, query, multi } of METRIC_QUERIES) {
        try {
          const res = await fetch(`/api/prometheus/query?q=${encodeURIComponent(query)}`);
          const data = await res.json();
          const results = data.data?.result ?? [];

          if (multi) {
            metrics.push({
              name,
              query,
              value: 0,
              series: results.map((r: { metric: Record<string, string>; value: [string, string] }) => ({
                label: r.metric.node_id?.replace('storage-node-', 'N') ?? '?',
                value: parseFloat(r.value[1]),
              })),
            });
          } else if (results[0]) {
            metrics.push({
              name,
              query,
              value: parseFloat(results[0].value[1]),
            });
          }
        } catch {
          /* prometheus unavailable */
        }
      }

      setMetrics(metrics);
    };

    poll();
    const id = setInterval(poll, 8000);
    return () => clearInterval(id);
  }, [serverOnline, setMetrics]);
}

export function useLogStream() {
  const serverOnline = useStore((s) => s.serverOnline);
  const addLog = useStore((s) => s.addLog);

  useEffect(() => {
    if (!serverOnline) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/logs`);

    ws.onmessage = (ev) => {
      try {
        const { container, line, ts } = JSON.parse(ev.data);
        addLog({ container, line, ts });
      } catch {
        /* ignore */
      }
    };

    return () => ws.close();
  }, [serverOnline, addLog]);
}

export function useGrafanaUrl() {
  return useCallback(async () => {
    const res = await fetch('/api/grafana/url');
    return res.json() as Promise<{ dashboard: string; embed: string; base: string }>;
  }, []);
}
