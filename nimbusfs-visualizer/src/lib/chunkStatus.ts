import type { SimPhase } from './constants';

export type ChunkStatus = 'pending' | 'hashing' | 'replicating' | 'stored';

export const CHUNK_STATUS_LABEL: Record<ChunkStatus, string> = {
  pending: 'Waiting',
  hashing: 'Hashing',
  replicating: 'Replicating',
  stored: 'Stored ✓',
};

export function getChunkStatus(
  chunkIndex: number,
  activeChunk: number,
  phase: SimPhase,
): ChunkStatus {
  if (phase === 'complete' || phase === 'metadata') {
    return 'stored';
  }

  if (phase === 'idle' || phase === 'uploading') {
    return 'pending';
  }

  if (chunkIndex < activeChunk) {
    return 'stored';
  }

  if (chunkIndex > activeChunk) {
    return 'pending';
  }

  // activeChunk === chunkIndex
  if (phase === 'chunking') return 'hashing';
  if (phase === 'replicating') return 'replicating';
  return 'pending';
}
