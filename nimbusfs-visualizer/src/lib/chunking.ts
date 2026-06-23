import { CHUNK_SIZE, REPLICATION_FACTOR, STORAGE_NODES, type ChunkPlan, type NodeId, type SimNode } from './constants';

function shuffle<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/** Mirrors replication.SelectNodesForChunk: top-N by free space, then shuffle. */
export function selectNodesForChunk(nodes: SimNode[], n = REPLICATION_FACTOR): NodeId[] {
  const alive = nodes.filter((node) => node.status === 'alive');
  if (alive.length < n) throw new Error('insufficient storage');

  const sorted = [...alive].sort((a, b) => {
    const freeA = a.totalSpace - a.usedSpace;
    const freeB = b.totalSpace - b.usedSpace;
    return freeB - freeA;
  });

  const top = sorted.slice(0, n);
  return shuffle(top).map((node) => node.id as NodeId);
}

export async function sha256Hex(data: ArrayBuffer): Promise<string> {
  const hash = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(hash))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

export async function planChunksFromFile(file: File, nodes: SimNode[]): Promise<ChunkPlan[]> {
  const buffer = await file.arrayBuffer();
  const total = buffer.byteLength;
  const chunkCount = Math.max(1, Math.ceil(total / CHUNK_SIZE));
  const plans: ChunkPlan[] = [];

  for (let i = 0; i < chunkCount; i++) {
    const start = i * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, total);
    const slice = buffer.slice(start, end);
    const chunkId = await sha256Hex(slice);
    const nodeIds = selectNodesForChunk(nodes);
    plans.push({ index: i, size: end - start, chunkId, nodeIds });
  }

  return plans;
}

export function defaultSimNodes(): SimNode[] {
  return STORAGE_NODES.map((n, i) => ({
    id: n.id,
    label: n.label,
    status: 'alive' as const,
    totalSpace: 10 * 1024 * 1024 * 1024,
    usedSpace: (1.2 + i * 0.4) * 1024 * 1024 * 1024,
  }));
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function truncateHash(hash: string, len = 12): string {
  return hash.length <= len ? hash : `${hash.slice(0, len)}…`;
}

export function nodePosition(angleDeg: number, radius = 5.5): [number, number, number] {
  const rad = (angleDeg * Math.PI) / 180;
  return [Math.cos(rad) * radius, -1.2, Math.sin(rad) * radius];
}

export const CHUNK_COLORS = ['#00f5ff', '#7b61ff', '#ff6bcb', '#ffd166', '#06d6a0', '#ef476f'];
