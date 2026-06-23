import { SceneCanvas } from './components/scene/SceneCanvas';
import {
  Header,
  UploadZone,
  ChunkBreakdown,
  EventTimeline,
} from './components/ui/Controls';
import { LogPanel, MetricsPanel } from './components/ui/SidePanels';
import { ServerStatusBanner } from './components/ui/ServerStatusBanner';
import { StoryGuide, WelcomeCard } from './components/ui/StoryGuide';
import { InsightsDashboard } from './components/ui/InsightsDashboard';
import {
  useServerHealth,
  useClusterSync,
  usePrometheusMetrics,
  useLogStream,
} from './hooks/useLiveData';

export function App() {
  useServerHealth();
  useClusterSync();
  usePrometheusMetrics();
  useLogStream();

  return (
    <div className="app">
      <ServerStatusBanner />

      <div className="scene-layer">
        <SceneCanvas />
      </div>

      <div className="ui-overlay ui-with-sticky-banner">
        <Header />

        <div className="layout">
          <aside className="left-col">
            <UploadZone />
            <ChunkBreakdown />
            <InsightsDashboard />
          </aside>

          <div className="center-col">
            <WelcomeCard />
            <StoryGuide />
          </div>

          <aside className="right-col">
            <LogPanel />
            <EventTimeline />
            <MetricsPanel />
          </aside>
        </div>
      </div>
    </div>
  );
}
