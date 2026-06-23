import { CHUNK_COLORS, formatBytes, truncateHash } from './chunking';
import type { ChunkPlan, SimPhase } from './constants';
import { useStore } from '../store/useStore';

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** ~2.5× slower than original — easier to follow. */
export const TIMING = {
  upload: 2000,
  streamGap: 1400,
  perChunk: 1400,
  placement: 900,
  packet: 2800,
  replicaPause: 700,
  metadata: 2000,
  done: 1000,
  replayUpload: 2800,
  replayChunk: 1600,
  replayPacket: 3000,
  replayMetadata: 2400,
  replayComplete: 1800,
} as const;

let abortGen = 0;

export function cancelReplay() {
  abortGen++;
}

function bumpAbort() {
  return ++abortGen;
}

function checkAbort(gen: number) {
  if (gen !== abortGen) throw new Error('aborted');
}

function animatePacket(id: string, durationMs: number, gen: number) {
  const start = performance.now();
  return new Promise<void>((resolve, reject) => {
    const tick = () => {
      if (gen !== abortGen) {
        reject(new Error('aborted'));
        return;
      }
      const t = Math.min(1, (performance.now() - start) / durationMs);
      useStore.getState().updatePacket(id, t);
      if (t < 1) requestAnimationFrame(tick);
      else resolve();
    };
    requestAnimationFrame(tick);
  });
}

/** Replay animation for a single pipeline step (after upload finished). */
export async function replayStep(stepId: SimPhase) {
  const store = useStore.getState();
  const { chunks, uploadResult } = store;
  if (chunks.length === 0) return;

  const gen = bumpAbort();
  const wasComplete = store.phase === 'complete' || !!uploadResult;

  useStore.setState({ isReplaying: true, packets: [], replayStep: stepId });

  try {
    switch (stepId) {
      case 'uploading':
        store.setPhase('uploading');
        store.setActiveChunk(-1);
        await sleep(TIMING.replayUpload);
        checkAbort(gen);
        break;

      case 'chunking':
        store.setPhase('chunking');
        for (const chunk of chunks) {
          checkAbort(gen);
          store.setActiveChunk(chunk.index);
          await sleep(TIMING.replayChunk);
        }
        checkAbort(gen);
        break;

      case 'replicating':
        for (const chunk of chunks) {
          checkAbort(gen);
          store.setActiveChunk(chunk.index);
          store.setPhase('replicating');
          const color = CHUNK_COLORS[chunk.index % CHUNK_COLORS.length];
          const packetIds = chunk.nodeIds.map((nodeId) =>
            store.addPacket({ chunkIndex: chunk.index, targetNodeId: nodeId, color }),
          );
          await Promise.all(packetIds.map((id) => animatePacket(id, TIMING.replayPacket, gen)));
          packetIds.forEach((id) => store.removePacket(id));
          await sleep(TIMING.replicaPause);
        }
        checkAbort(gen);
        break;

      case 'metadata':
        store.setPhase('metadata');
        store.setActiveChunk(chunks.length - 1);
        await sleep(TIMING.replayMetadata);
        checkAbort(gen);
        break;

      case 'complete':
        store.setPhase('complete');
        store.setActiveChunk(chunks.length - 1);
        await sleep(TIMING.replayComplete);
        checkAbort(gen);
        break;

      default:
        break;
    }
  } catch {
    /* replay cancelled */
  } finally {
    if (gen === abortGen) {
      useStore.setState({
        isReplaying: false,
        replayStep: null,
        packets: [],
        phase: wasComplete ? 'complete' : store.phase,
        activeChunkIndex: wasComplete ? chunks.length - 1 : store.activeChunkIndex,
      });
    }
  }
}

/** Demo-mode cinematic playback of the upload pipeline. */
export async function runDemoSimulation(
  fileName: string,
  fileSize: number,
  chunks: ChunkPlan[],
) {
  const gen = bumpAbort();
  const store = useStore.getState();
  store.startSimulation(fileName, fileSize, chunks);

  store.addEvent('upload', `Your file "${fileName}" is sent to the Master (${formatBytes(fileSize)})`);
  store.setPhase('uploading');
  await sleep(TIMING.upload);
  checkAbort(gen);

  store.addEvent('stream', 'Master streams bytes — never loads the whole file into memory');
  store.setPhase('chunking');
  await sleep(TIMING.streamGap);
  checkAbort(gen);

  for (const chunk of chunks) {
    store.setActiveChunk(chunk.index);
    store.setPhase('chunking');
    store.addEvent(
      'chunk',
      `Slice #${chunk.index}: ${formatBytes(chunk.size)} → fingerprint ${truncateHash(chunk.chunkId)}`,
      { chunkId: chunk.chunkId, index: chunk.index },
    );
    await sleep(TIMING.perChunk);
    checkAbort(gen);

    store.addEvent(
      'placement',
      `Picking 3 nodes with most free disk → ${chunk.nodeIds.map(shortNode).join(', ')}`,
      { nodeIds: chunk.nodeIds },
    );
    store.setPhase('replicating');

    const color = CHUNK_COLORS[chunk.index % CHUNK_COLORS.length];
    const packetIds = chunk.nodeIds.map((nodeId) =>
      store.addPacket({ chunkIndex: chunk.index, targetNodeId: nodeId, color }),
    );

    await Promise.all(packetIds.map((id) => animatePacket(id, TIMING.packet, gen)));

    packetIds.forEach((id) => store.removePacket(id));
    store.addEvent('replica', `Chunk ${chunk.index} safely stored on 3 machines`);
    await sleep(TIMING.replicaPause);
    checkAbort(gen);
  }

  store.setPhase('metadata');
  store.addEvent('bolt', 'Master saves the map (file → chunks → nodes) in BoltDB');
  await sleep(TIMING.metadata);
  checkAbort(gen);

  store.addEvent('done', `Done! ${chunks.length} chunks × 3 copies — file survives node failures`);
  store.completeSimulation({
    fileId: crypto.randomUUID(),
    chunks: chunks.length,
    size: fileSize,
  });
}

/** Live mode: animate while upload runs, then sync with server metadata. */
export async function runLiveSimulation(
  fileName: string,
  fileSize: number,
  predictedChunks: ChunkPlan[],
  uploadPromise: Promise<{ file_id: string; chunks: number; size: number }>,
) {
  const gen = bumpAbort();
  const store = useStore.getState();
  store.startSimulation(fileName, fileSize, predictedChunks);

  store.addEvent('upload', `[REAL] Uploading "${fileName}" to your NimbusFS cluster…`);
  store.setPhase('uploading');

  const uploadTask = uploadPromise.then(async (result) => {
    try {
      const res = await fetch('/api/files?limit=50');
      const data = await res.json();
      const file = data.files?.find((f: { FileID: string }) => f.FileID === result.file_id);
      if (file?.Chunks) {
        const realChunks: ChunkPlan[] = file.Chunks.map(
          (c: { Index: number; Size: number; ChunkID: string; NodeIDs: string[] }) => ({
            index: c.Index,
            size: c.Size,
            chunkId: c.ChunkID,
            nodeIds: c.NodeIDs,
          }),
        );
        useStore.setState({ chunks: realChunks });
        store.addEvent('sync', 'Confirmed real placement from cluster metadata');
      }
    } catch {
      store.addEvent('sync', 'Using predicted placement (metadata fetch skipped)');
    }
    return result;
  });

  store.setPhase('chunking');
  for (const chunk of predictedChunks) {
    checkAbort(gen);
    store.setActiveChunk(chunk.index);
    store.addEvent('chunk', `Chunk ${chunk.index} hashed → ${truncateHash(chunk.chunkId)}`);
    await sleep(TIMING.perChunk);

    store.setPhase('replicating');
    const color = CHUNK_COLORS[chunk.index % CHUNK_COLORS.length];
    const ids = chunk.nodeIds.map((nodeId) =>
      store.addPacket({ chunkIndex: chunk.index, targetNodeId: nodeId, color }),
    );
    await Promise.all(ids.map((id) => animatePacket(id, TIMING.packet, gen)));
    ids.forEach((id) => store.removePacket(id));
    store.addEvent('replica', `Chunk ${chunk.index} written to 3 storage nodes via gRPC`);
    await sleep(TIMING.replicaPause);
    checkAbort(gen);
  }

  store.setPhase('metadata');
  const result = await uploadTask;
  checkAbort(gen);
  store.addEvent('bolt', `Stored on cluster — file_id: ${result.file_id.slice(0, 8)}…`);
  store.completeSimulation({
    fileId: result.file_id,
    chunks: result.chunks,
    size: result.size,
  });
}

function shortNode(id: string): string {
  return id.replace('storage-node-', 'Node ');
}
