import { create } from 'zustand';
import type {
  ChunkPlan,
  DataPacket,
  LogEntry,
  PrometheusMetric,
  SimEvent,
  SimNode,
  SimPhase,
} from '../lib/constants';
import { defaultSimNodes } from '../lib/chunking';

interface VisualizerState {
  serverOnline: boolean;
  healthChecked: boolean;
  isSimulating: boolean;
  fileName: string | null;
  fileSize: number;
  chunks: ChunkPlan[];
  activeChunkIndex: number;
  nodes: SimNode[];
  packets: DataPacket[];
  events: SimEvent[];
  logs: LogEntry[];
  metrics: PrometheusMetric[];
  phase: SimPhase;
  uploadResult: { fileId: string; chunks: number; size: number } | null;
  isReplaying: boolean;
  replayStep: SimPhase | null;

  setServerOnline: (online: boolean) => void;
  setHealthChecked: (checked: boolean) => void;
  setNodes: (nodes: SimNode[]) => void;
  setMetrics: (metrics: PrometheusMetric[]) => void;
  addLog: (entry: Omit<LogEntry, 'id'>) => void;
  addEvent: (type: string, message: string, meta?: Record<string, unknown>) => void;
  setPhase: (phase: VisualizerState['phase']) => void;
  startSimulation: (fileName: string, fileSize: number, chunks: ChunkPlan[]) => void;
  setActiveChunk: (index: number) => void;
  addPacket: (packet: Omit<DataPacket, 'id' | 'progress'>) => string;
  updatePacket: (id: string, progress: number) => void;
  removePacket: (id: string) => void;
  completeSimulation: (result?: { fileId: string; chunks: number; size: number }) => void;
  reset: () => void;
}

let eventCounter = 0;
let logCounter = 0;
let packetCounter = 0;

export const useStore = create<VisualizerState>((set) => ({
  serverOnline: false,
  healthChecked: false,
  isSimulating: false,
  fileName: null,
  fileSize: 0,
  chunks: [],
  activeChunkIndex: -1,
  nodes: defaultSimNodes(),
  packets: [],
  events: [],
  logs: [],
  metrics: [],
  phase: 'idle',
  uploadResult: null,
  isReplaying: false,
  replayStep: null,

  setServerOnline: (online) => set({ serverOnline: online }),
  setHealthChecked: (checked) => set({ healthChecked: checked }),
  setNodes: (nodes) => set({ nodes }),
  setMetrics: (metrics) => set({ metrics }),

  addLog: (entry) =>
    set((s) => ({
      logs: [
        { ...entry, id: `log-${++logCounter}` },
        ...s.logs,
      ].slice(0, 200),
    })),

  addEvent: (type, message, meta) =>
    set((s) => ({
      events: [
        { id: `evt-${++eventCounter}`, type, message, ts: Date.now(), meta },
        ...s.events,
      ].slice(0, 80),
    })),

  setPhase: (phase) => set({ phase }),

  startSimulation: (fileName, fileSize, chunks) =>
    set({
      isSimulating: true,
      fileName,
      fileSize,
      chunks,
      activeChunkIndex: -1,
      packets: [],
      events: [],
      uploadResult: null,
      phase: 'uploading',
    }),

  setActiveChunk: (index) => set({ activeChunkIndex: index }),

  addPacket: (packet) => {
    const id = `pkt-${++packetCounter}`;
    set((s) => ({
      packets: [...s.packets, { ...packet, id, progress: 0 }],
    }));
    return id;
  },

  updatePacket: (id, progress) =>
    set((s) => ({
      packets: s.packets.map((p) => (p.id === id ? { ...p, progress } : p)),
    })),

  removePacket: (id) =>
    set((s) => ({
      packets: s.packets.filter((p) => p.id !== id),
    })),

  completeSimulation: (result) =>
    set((s) => ({
      isSimulating: false,
      isReplaying: false,
      replayStep: null,
      phase: 'complete' as SimPhase,
      uploadResult: result ?? null,
      activeChunkIndex: s.chunks.length > 0 ? s.chunks.length - 1 : -1,
    })),

  reset: () =>
    set({
      isSimulating: false,
      fileName: null,
      fileSize: 0,
      chunks: [],
      activeChunkIndex: -1,
      packets: [],
      events: [],
      phase: 'idle',
      uploadResult: null,
    }),
}));
