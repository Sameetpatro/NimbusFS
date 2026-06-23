import { useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useStore } from '../../store/useStore';
import { formatBytes } from '../../lib/chunking';
import { useFileUpload } from '../../hooks/useFileUpload';
import { createDummyFile, DUMMY_FILE_META } from '../../lib/dummyFile';
import { CHUNK_STATUS_LABEL, getChunkStatus } from '../../lib/chunkStatus';

const PHASE_LABELS: Record<string, string> = {
  idle: 'Standby',
  uploading: 'Receiving stream',
  chunking: 'Chunking + SHA-256',
  replicating: 'gRPC replication',
  metadata: 'BoltDB write',
  complete: 'Complete',
};

export function Header() {
  const serverOnline = useStore((s) => s.serverOnline);
  const phase = useStore((s) => s.phase);

  return (
    <header className="header header-with-banner">
      <div className="header-brand">
        <motion.h1
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          className="title"
        >
          NIMBUS<span>FS</span>
        </motion.h1>
        <p className="subtitle">Distributed Storage Visualizer</p>
      </div>
      <div className="header-status">
        <div className="phase-badge">{PHASE_LABELS[phase] ?? phase}</div>
        {serverOnline && <div className="phase-badge live-tag">REAL UPLOADS</div>}
      </div>
    </header>
  );
}

export function UploadZone() {
  const { handleFile, isSimulating } = useFileUpload();
  const serverOnline = useStore((s) => s.serverOnline);
  const phase = useStore((s) => s.phase);
  const fileName = useStore((s) => s.fileName);
  const fileSize = useStore((s) => s.fileSize);
  const chunks = useStore((s) => s.chunks);
  const uploadResult = useStore((s) => s.uploadResult);

  const pendingDummy = useRef<File | null>(null);

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    if (e.dataTransfer.getData('application/nimbusfs-dummy') === '1') {
      handleFile(pendingDummy.current ?? createDummyFile());
      pendingDummy.current = null;
      return;
    }
    const file = e.dataTransfer.files[0];
    if (file) handleFile(file);
  };

  const startDummy = () => handleFile(createDummyFile());

  const onDummyDragStart = (e: React.DragEvent) => {
    pendingDummy.current = createDummyFile();
    e.dataTransfer.setData('application/nimbusfs-dummy', '1');
    e.dataTransfer.setData('text/plain', DUMMY_FILE_META.name);
    e.dataTransfer.effectAllowed = 'copy';
  };

  return (
    <div className="upload-zone glass">
      <div className="upload-zone-header">
        <h3>Upload</h3>
        <span className={`upload-mode-tag ${serverOnline ? 'live' : 'demo'}`}>
          {serverOnline ? 'Real cluster' : 'Demo animation'}
        </span>
      </div>

      <div className={`upload-pair ${!serverOnline ? 'with-dummy' : ''}`}>
        <div
          className={`drop-area ${isSimulating ? 'active' : ''}`}
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
        >
          <motion.div
            className="drop-icon"
            animate={{ y: isSimulating ? [0, -8, 0] : 0 }}
            transition={{ repeat: isSimulating ? Infinity : 0, duration: 1.8 }}
          >
            {serverOnline ? '📁' : '↓'}
          </motion.div>
          <p>
            {serverOnline
              ? 'Drop your file here — stored on real storage nodes'
              : 'Drop zone — drag the sample file here'}
          </p>
          {serverOnline && (
            <label className="file-btn">
              Browse file
              <input
                type="file"
                hidden
                disabled={isSimulating}
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) handleFile(f);
                  e.target.value = '';
                }}
              />
            </label>
          )}
        </div>

        {!serverOnline && (
          <DummyFileCard
            isSimulating={isSimulating}
            onDragStart={onDummyDragStart}
            onClick={startDummy}
          />
        )}
      </div>

      <AnimatePresence>
        {fileName && (
          <motion.div
            className="file-info"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
          >
            <div className="file-row">
              <span className="label">File</span>
              <span>{fileName}</span>
            </div>
            <div className="file-row">
              <span className="label">Size</span>
              <span>{formatBytes(fileSize)}</span>
            </div>
            <div className="file-row">
              <span className="label">Chunks</span>
              <span>{chunks.length} (4 MiB each, last may be smaller)</span>
            </div>
            {uploadResult && (
              <div className="file-row success">
                <span className="label">file_id</span>
                <span className="mono">{uploadResult.fileId.slice(0, 18)}…</span>
              </div>
            )}
            {isSimulating && (
              <div className="progress-bar">
                <motion.div
                  className="progress-fill"
                  animate={{ width: phaseWidth(phase) }}
                  transition={{ duration: 0.6 }}
                />
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function DummyFileCard({
  isSimulating,
  onDragStart,
  onClick,
}: {
  isSimulating: boolean;
  onDragStart: (e: React.DragEvent) => void;
  onClick: () => void;
}) {
  return (
    <motion.div animate={{ y: [0, -4, 0] }} transition={{ repeat: Infinity, duration: 3, ease: 'easeInOut' }}>
      <div
        className="dummy-file-card"
        draggable={!isSimulating}
        onDragStart={onDragStart}
        onClick={!isSimulating ? onClick : undefined}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (!isSimulating && (e.key === 'Enter' || e.key === ' ')) onClick();
        }}
      >
        <div className="dummy-icon">📦</div>
        <div className="dummy-name">{DUMMY_FILE_META.name}</div>
        <div className="dummy-size">{DUMMY_FILE_META.sizeLabel}</div>
        <div className="dummy-chunks">{DUMMY_FILE_META.chunks} chunks · RF=3</div>
        <div className="dummy-hint">Drag me → or click</div>
      </div>
    </motion.div>
  );
}

function phaseWidth(phase: string): string {
  const map: Record<string, string> = {
    uploading: '15%',
    chunking: '35%',
    replicating: '70%',
    metadata: '90%',
    complete: '100%',
  };
  return map[phase] ?? '0%';
}

export function ChunkBreakdown() {
  const chunks = useStore((s) => s.chunks);
  const activeChunk = useStore((s) => s.activeChunkIndex);
  const phase = useStore((s) => s.phase);

  if (chunks.length === 0) return null;

  const storedCount = chunks.filter((c) => getChunkStatus(c.index, activeChunk, phase) === 'stored').length;

  return (
    <div className="chunk-breakdown glass">
      <div className="chunk-breakdown-header">
        <h3>Chunk status</h3>
        <span className="chunk-summary">
          {storedCount}/{chunks.length} stored
        </span>
      </div>
      <p className="panel-explainer">
        Each chunk moves: Waiting → Hashing → Replicating → Stored on 3 nodes.
      </p>
      <div className="chunk-list">
        {chunks.map((chunk) => {
          const status = getChunkStatus(chunk.index, activeChunk, phase);

          return (
            <motion.div
              key={chunk.index}
              className={`chunk-card status-${status}`}
              layout
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: chunk.index * 0.05 }}
            >
              <div className="chunk-header">
                <span className="chunk-idx">Chunk #{chunk.index}</span>
                <span className={`chunk-status-pill status-${status}`}>
                  {CHUNK_STATUS_LABEL[status]}
                </span>
              </div>
              <div className="chunk-meta-row">
                <span className="chunk-size">{formatBytes(chunk.size)}</span>
              </div>
              <div className="chunk-hash mono">{chunk.chunkId.slice(0, 20)}…</div>
              <div className="replica-tags">
                {chunk.nodeIds.map((nid: string) => (
                  <span
                    key={nid}
                    className={`replica-tag ${status === 'stored' ? 'replica-done' : ''}`}
                  >
                    {nid.replace('storage-node-', 'Node ')}
                  </span>
                ))}
              </div>
            </motion.div>
          );
        })}
      </div>
    </div>
  );
}

export function EventTimeline() {
  const events = useStore((s) => s.events);

  return (
    <div className="event-timeline glass side-panel">
      <h3>Technical log</h3>
      <p className="panel-explainer">Step-by-step API and system events from the upload.</p>
      <div className="events-scroll">
        <AnimatePresence initial={false}>
          {events.map((ev) => (
            <motion.div
              key={ev.id}
              className={`event-item type-${ev.type}`}
              initial={{ opacity: 0, x: 30, height: 0 }}
              animate={{ opacity: 1, x: 0, height: 'auto' }}
              exit={{ opacity: 0 }}
            >
              <span className="event-type">{ev.type}</span>
              <span className="event-msg">{ev.message}</span>
            </motion.div>
          ))}
        </AnimatePresence>
        {events.length === 0 && (
          <p className="empty">Events appear here as your file moves through the system</p>
        )}
      </div>
    </div>
  );
}
