import { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useStore } from '../../store/useStore';

type BannerPhase = 'waiting' | 'flash' | 'sticky';

export function ServerStatusBanner() {
  const serverOnline = useStore((s) => s.serverOnline);
  const healthChecked = useStore((s) => s.healthChecked);
  const [phase, setPhase] = useState<BannerPhase>('waiting');
  const flashed = useRef(false);

  useEffect(() => {
    if (!healthChecked || flashed.current) return;
    flashed.current = true;
    setPhase('flash');
    const t = setTimeout(() => setPhase('sticky'), 2400);
    return () => clearTimeout(t);
  }, [healthChecked]);

  const text = serverOnline ? 'SERVER IS RUNNING LIVE' : 'SERVER SOO RAHA HAI';
  const sub = serverOnline
    ? 'NimbusFS cluster detected — uploads hit the real cluster'
    : 'Cluster is asleep — drag the sample file to see how it works';

  if (phase === 'waiting') return null;

  return (
    <>
      <AnimatePresence>
        {phase === 'flash' && (
          <motion.div
            className="status-flash-overlay"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.25 }}
          >
            <motion.div
              className={`status-flash-card ${serverOnline ? 'live' : 'sleep'}`}
              initial={{ scale: 0.85, y: 30 }}
              animate={{ scale: 1, y: 0 }}
              exit={{ scale: 0.6, y: -120, opacity: 0 }}
              transition={{ type: 'spring', stiffness: 280, damping: 22 }}
            >
              <motion.span
                className="flash-dot"
                animate={{ scale: [1, 1.4, 1], opacity: [1, 0.5, 1] }}
                transition={{ repeat: Infinity, duration: 0.8 }}
              />
              <h2>{text}</h2>
              <p>{sub}</p>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {phase === 'sticky' && (
        <motion.div
          className={`status-sticky ${serverOnline ? 'live' : 'sleep'}`}
          initial={{ y: -80, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ type: 'spring', stiffness: 320, damping: 28 }}
        >
          <span className="sticky-dot" />
          <strong>{text}</strong>
          <span className="sticky-sub">{serverOnline ? '· live cluster' : '· demo mode'}</span>
        </motion.div>
      )}
    </>
  );
}
