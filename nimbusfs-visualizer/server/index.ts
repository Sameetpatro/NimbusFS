import express from 'express';
import cors from 'cors';
import multer from 'multer';
import { spawn, exec } from 'child_process';
import { createServer } from 'http';
import { WebSocketServer, WebSocket } from 'ws';
import { promisify } from 'util';

const execAsync = promisify(exec);

const app = express();
const server = createServer(app);
const wss = new WebSocketServer({ server, path: '/ws/logs' });

const PORT = 4000;
const MASTER_URL = process.env.NIMBUSFS_MASTER_URL ?? 'http://localhost:8080';
const API_KEY = process.env.NIMBUSFS_API_KEY ?? 'demo-key';
const PROMETHEUS_URL = process.env.PROMETHEUS_URL ?? 'http://localhost:9099';
const GRAFANA_URL = process.env.GRAFANA_URL ?? 'http://localhost:3000';

const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 64 * 1024 * 1024 } });

app.use(cors());
app.use(express.json());

const DOCKER_CONTAINERS = [
  'dfs-master',
  'dfs-storage-1',
  'dfs-storage-2',
  'dfs-storage-3',
  'dfs-storage-4',
  'dfs-storage-5',
];

async function nimbusFetch(path: string, init?: RequestInit) {
  const headers = new Headers(init?.headers);
  if (!headers.has('X-API-Key')) {
    headers.set('X-API-Key', API_KEY);
  }
  return fetch(`${MASTER_URL}${path}`, { ...init, headers });
}

app.get('/api/health', async (_req, res) => {
  try {
    const r = await fetch(`${MASTER_URL}/api/v1/health`, { signal: AbortSignal.timeout(3000) });
    const data = await r.json();
    res.json({ online: r.ok, master: data, url: MASTER_URL });
  } catch {
    res.json({ online: false, master: null, url: MASTER_URL });
  }
});

app.get('/api/cluster/status', async (_req, res) => {
  try {
    const r = await nimbusFetch('/api/v1/cluster/status');
    const data = await r.json();
    res.status(r.status).json(data);
  } catch (e) {
    res.status(502).json({ error: String(e) });
  }
});

app.get('/api/files', async (req, res) => {
  try {
    const page = req.query.page ?? '1';
    const limit = req.query.limit ?? '50';
    const r = await nimbusFetch(`/api/v1/files?page=${page}&limit=${limit}`);
    const data = await r.json();
    res.status(r.status).json(data);
  } catch (e) {
    res.status(502).json({ error: String(e) });
  }
});

app.post('/api/upload', upload.single('file'), async (req, res) => {
  if (!req.file) {
    res.status(400).json({ error: 'no file' });
    return;
  }
  try {
    const form = new FormData();
    const blob = new Blob([req.file.buffer], { type: req.file.mimetype });
    form.append('file', blob, req.file.originalname);

    const r = await nimbusFetch('/api/v1/upload', {
      method: 'POST',
      headers: { 'X-API-Key': API_KEY },
      body: form,
    });
    const data = await r.json();
    res.status(r.status).json(data);
  } catch (e) {
    res.status(502).json({ error: String(e) });
  }
});

app.get('/api/prometheus/query', async (req, res) => {
  const query = req.query.q as string;
  if (!query) {
    res.status(400).json({ error: 'missing q' });
    return;
  }
  try {
    const url = `${PROMETHEUS_URL}/api/v1/query?query=${encodeURIComponent(query)}`;
    const r = await fetch(url, { signal: AbortSignal.timeout(5000) });
    const data = await r.json();
    res.status(r.status).json(data);
  } catch {
    res.json({ status: 'error', data: { result: [] } });
  }
});

app.get('/api/prometheus/query_range', async (req, res) => {
  const query = req.query.q as string;
  const start = req.query.start as string;
  const end = req.query.end as string;
  const step = req.query.step ?? '15';
  if (!query || !start || !end) {
    res.status(400).json({ error: 'missing params' });
    return;
  }
  try {
    const url = `${PROMETHEUS_URL}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${start}&end=${end}&step=${step}`;
    const r = await fetch(url, { signal: AbortSignal.timeout(5000) });
    const data = await r.json();
    res.status(r.status).json(data);
  } catch {
    res.json({ status: 'error', data: { result: [] } });
  }
});

app.get('/api/grafana/url', (_req, res) => {
  res.json({
    dashboard: `${GRAFANA_URL}/d/nimbusfs-dashboard/nimbusfs-dashboard?orgId=1&refresh=5s&kiosk`,
    embed: `${GRAFANA_URL}/d-solo/nimbusfs-dashboard/nimbusfs-dashboard?orgId=1&panelId=1&refresh=5s&theme=dark`,
    base: GRAFANA_URL,
  });
});

app.get('/api/docker/available', async (_req, res) => {
  try {
    await execAsync('docker info', { timeout: 3000 });
    res.json({ available: true });
  } catch {
    res.json({ available: false });
  }
});

// WebSocket log streaming from docker containers
const logProcesses = new Map<WebSocket, ReturnType<typeof spawn>[]>();

wss.on('connection', (ws) => {
  const procs: ReturnType<typeof spawn>[] = [];

  const send = (container: string, line: string) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ container, line, ts: Date.now() }));
    }
  };

  for (const container of DOCKER_CONTAINERS) {
    const proc = spawn('docker', ['logs', '-f', '--tail', '20', container], {
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    proc.stdout?.on('data', (buf: Buffer) => {
      buf.toString().split('\n').filter(Boolean).forEach((line) => send(container, line));
    });
    proc.stderr?.on('data', (buf: Buffer) => {
      buf.toString().split('\n').filter(Boolean).forEach((line) => send(container, line));
    });
    proc.on('error', () => send(container, `[${container}] docker logs unavailable`));

    procs.push(proc);
  }

  logProcesses.set(ws, procs);

  ws.on('close', () => {
    procs.forEach((p) => p.kill('SIGTERM'));
    logProcesses.delete(ws);
  });
});

server.listen(PORT, () => {
  console.log(`NimbusFS visualizer proxy → ${MASTER_URL} on :${PORT}`);
});
