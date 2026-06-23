import { useRef, useCallback } from 'react';
import { useStore } from '../store/useStore';
import { planChunksFromFile } from '../lib/chunking';
import { runDemoSimulation, runLiveSimulation, cancelReplay } from '../lib/simulation';

export function useFileUpload() {
  const serverOnline = useStore((s) => s.serverOnline);
  const nodes = useStore((s) => s.nodes);
  const isSimulating = useStore((s) => s.isSimulating);
  const reset = useStore((s) => s.reset);
  const running = useRef(false);

  const handleFile = useCallback(
    async (file: File) => {
      if (running.current || isSimulating) return;
      running.current = true;
      cancelReplay();
      reset();

      try {
        const chunks = await planChunksFromFile(file, nodes);

        if (serverOnline) {
          const form = new FormData();
          form.append('file', file);
          const uploadPromise = fetch('/api/upload', { method: 'POST', body: form }).then((r) => {
            if (!r.ok) throw new Error('upload failed');
            return r.json();
          });
          await runLiveSimulation(file.name, file.size, chunks, uploadPromise);
        } else {
          await runDemoSimulation(file.name, file.size, chunks);
        }
      } catch (e) {
        useStore.getState().addEvent('error', String(e));
        useStore.getState().completeSimulation();
      } finally {
        running.current = false;
      }
    },
    [serverOnline, nodes, isSimulating, reset],
  );

  return { handleFile, isSimulating };
}
