export const CHUNK_SIZE = 4 * 1024 * 1024; // 4 MiB — matches NimbusFS config
export const REPLICATION_FACTOR = 3;

export const STORAGE_NODES = [
  { id: 'storage-node-1', label: 'Node 1', port: 9091, angle: 0 },
  { id: 'storage-node-2', label: 'Node 2', port: 9092, angle: 72 },
  { id: 'storage-node-3', label: 'Node 3', port: 9093, angle: 144 },
  { id: 'storage-node-4', label: 'Node 4', port: 9094, angle: 216 },
  { id: 'storage-node-5', label: 'Node 5', port: 9095, angle: 288 },
] as const;

export type NodeId = (typeof STORAGE_NODES)[number]['id'];

export interface ChunkPlan {
  index: number;
  size: number;
  chunkId: string;
  nodeIds: NodeId[];
}

export interface SimNode {
  id: string;
  label: string;
  status: 'alive' | 'suspect' | 'dead';
  totalSpace: number;
  usedSpace: number;
}

export interface LogEntry {
  id: string;
  container: string;
  line: string;
  ts: number;
}

export interface DataPacket {
  id: string;
  chunkIndex: number;
  targetNodeId: string;
  progress: number;
  color: string;
}

export interface SimEvent {
  id: string;
  type: string;
  message: string;
  ts: number;
  meta?: Record<string, unknown>;
}

export type AppMode = 'demo' | 'live';

export type SimPhase = 'idle' | 'uploading' | 'chunking' | 'replicating' | 'metadata' | 'complete';

export interface PrometheusMetric {
  name: string;
  query: string;
  value: number;
  series?: { label: string; value: number }[];
}
