import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.DFS_SERVER || 'http://localhost:8080';
const API_KEY = __ENV.DFS_API_KEY || 'demo-key';

export const options = {
  scenarios: {
    upload_load: {
      executor: 'constant-vus',
      vus: 100,
      duration: '30s',
    },
    download_load: {
      executor: 'constant-vus',
      vus: 100,
      duration: '30s',
      startTime: '35s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.01'],
  },
};

const uploadedIDs = [];

export function setup() {
  const payload = open('./payload-1mb.bin', 'b');
  const data = {
    file: http.file(payload, 'payload-1mb.bin'),
  };
  const res = http.post(`${BASE}/api/v1/upload`, data, {
    headers: { 'X-API-Key': API_KEY },
  });
  if (res.status === 201) {
    const body = JSON.parse(res.body);
    return [body.file_id];
  }
  return [];
}

export default function (data) {
  const fileId = data && data[0];
  if (__ITER % 2 === 0 || !fileId) {
    const bin = new Uint8Array(1024 * 1024);
    for (let i = 0; i < bin.length; i++) bin[i] = i % 256;
    const res = http.post(`${BASE}/api/v1/upload`, {
      file: http.file(bin.buffer, `load-${__VU}-${__ITER}.bin`),
    }, { headers: { 'X-API-Key': API_KEY } });
    check(res, { 'upload ok': (r) => r.status === 201 });
  } else {
    const res = http.get(`${BASE}/api/v1/files/${fileId}/download`, {
      headers: { 'X-API-Key': API_KEY },
    });
    check(res, { 'download ok': (r) => r.status === 200 || r.status === 206 });
  }
  sleep(0.1);
}
