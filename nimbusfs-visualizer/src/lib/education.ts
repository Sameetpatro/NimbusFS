import type { SimPhase } from './constants';

export const PIPELINE_STEPS = [
  { id: 'uploading', step: 1, title: 'Upload', icon: '📤' },
  { id: 'chunking', step: 2, title: 'Chunking', icon: '✂️' },
  { id: 'replicating', step: 3, title: 'Replication', icon: '🔁' },
  { id: 'metadata', step: 4, title: 'Catalog', icon: '📋' },
  { id: 'complete', step: 5, title: 'Done', icon: '✅' },
] as const;

export const PHASE_EXPLANATIONS: Record<
  SimPhase,
  { title: string; body: string; detail: string }
> = {
  idle: {
    title: 'Welcome to NimbusFS',
    body:
      'This is a distributed file system — like Google Drive under the hood, but you own every piece.',
    detail:
      'Drop a file (or the sample file when offline) to watch it travel through the cluster.',
  },
  uploading: {
    title: 'Step 1 — Your file reaches the Master',
    body:
      'The client sends your file to the Master node over HTTP. The Master never keeps the whole file in RAM — it streams it.',
    detail:
      'Think of the Master as a librarian: it receives the book but stores copies on shelves (storage nodes), not at the front desk.',
  },
  chunking: {
    title: 'Step 2 — Split into 4 MiB chunks',
    body:
      'Big files are sliced into fixed 4 MiB pieces. Each chunk gets a unique ID from its SHA-256 hash (content-addressed).',
    detail:
      'A 10 MB file becomes 3 chunks: two full 4 MB blocks + one smaller tail. Same content always produces the same chunk ID.',
  },
  replicating: {
    title: 'Step 3 — 3 copies on different machines',
    body:
      'Every chunk is copied to 3 storage nodes in parallel via gRPC. If one machine dies, two copies still survive.',
    detail:
      'Nodes are picked by free disk space, then shuffled — so one node doesn’t absorb every upload.',
  },
  metadata: {
    title: 'Step 4 — Catalog saved in BoltDB',
    body:
      'The Master writes metadata: file name, size, and which nodes hold each chunk. No bytes stored here — only the map.',
    detail:
      'BoltDB is an embedded key-value store on the Master. Downloads use this catalog to find and reassemble chunks.',
  },
  complete: {
    title: 'Step 5 — File is durable & retrievable',
    body:
      'Your file is now spread across the cluster with 3× redundancy. You can download it anytime — even if a node fails.',
    detail:
      'The file_id in the response is your handle. The cluster can heal missing replicas in the background automatically.',
  },
};

export function getStepIndex(phase: SimPhase): number {
  const idx = PIPELINE_STEPS.findIndex((s) => s.id === phase);
  return idx >= 0 ? idx : 0;
}
