import { motion, AnimatePresence } from 'framer-motion';
import { useStore } from '../../store/useStore';
import { PHASE_EXPLANATIONS, PIPELINE_STEPS, getStepIndex } from '../../lib/education';
import { replayStep, cancelReplay } from '../../lib/simulation';
import type { SimPhase } from '../../lib/constants';

export function StoryGuide() {
  const phase = useStore((s) => s.phase);
  const isSimulating = useStore((s) => s.isSimulating);
  const isReplaying = useStore((s) => s.isReplaying);
  const replayingStep = useStore((s) => s.replayStep);
  const serverOnline = useStore((s) => s.serverOnline);
  const chunks = useStore((s) => s.chunks);
  const info = PHASE_EXPLANATIONS[isReplaying && replayingStep ? replayingStep : phase];
  const stepIdx = getStepIndex(isReplaying && replayingStep ? replayingStep : phase);
  const canReplay = chunks.length > 0 && !isSimulating;

  const onStepClick = (stepId: SimPhase) => {
    if (!canReplay && stepId !== 'uploading') return;
    if (chunks.length === 0) return;
    cancelReplay();
    void replayStep(stepId);
  };

  return (
    <div className="story-guide glass">
      <p className="story-tap-hint">
        {canReplay ? 'Tap a step to replay its animation' : 'Upload a file first, then tap steps to replay'}
      </p>
      <div className="pipeline-steps">
        {PIPELINE_STEPS.map((step, i) => {
          const isActive =
            (isReplaying && replayingStep === step.id) ||
            (!isReplaying && i === stepIdx && (isSimulating || phase === 'complete'));
          const isDone = i < stepIdx || phase === 'complete';

          return (
            <button
              key={step.id}
              type="button"
              className={`pipeline-step ${isDone ? 'done' : ''} ${isActive ? 'active' : ''}`}
              onClick={() => onStepClick(step.id)}
              disabled={!canReplay}
              title={canReplay ? `Replay: ${step.title}` : 'Complete an upload first'}
            >
              <span className="step-icon">{step.icon}</span>
              <span className="step-label">{step.title}</span>
            </button>
          );
        })}
      </div>

      <AnimatePresence mode="wait">
        <motion.div
          key={(isReplaying ? replayingStep : phase) + String(isSimulating)}
          className="story-content"
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -8 }}
          transition={{ duration: 0.35 }}
        >
          <div className="story-badge">
            {isReplaying ? 'REPLAY' : serverOnline ? 'LIVE' : 'DEMO'}
          </div>
          <h3>{info.title}</h3>
          <p className="story-body">{info.body}</p>
          <p className="story-detail">{info.detail}</p>
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

export function WelcomeCard() {
  const phase = useStore((s) => s.phase);
  const serverOnline = useStore((s) => s.serverOnline);

  if (phase !== 'idle') return null;

  return (
    <motion.div
      className="welcome-card glass"
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
    >
      <h3>What is happening here?</h3>
      <p>
        <strong>NimbusFS</strong> stores files across multiple machines — not on one disk.
        When you upload, the file is <em>chunked</em>, <em>replicated 3×</em>, and tracked by a{' '}
        <em>master catalog</em>.
      </p>
      <ul>
        <li><strong>Master</strong> — brain: API, metadata, placement decisions</li>
        <li><strong>5 Storage nodes</strong> — muscles: hold actual chunk bytes</li>
        <li><strong>3 replicas</strong> — each chunk survives 2 node failures</li>
      </ul>
      <p className="welcome-cta">
        {serverOnline
          ? '↑ Drop any real file to upload into your running cluster'
          : '↑ Drag the sample file (vacation-photos.zip, 10 MB, 3 chunks)'}
      </p>
    </motion.div>
  );
}
