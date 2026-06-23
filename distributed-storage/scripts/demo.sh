#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "=== Distributed File Storage System Demo ==="

echo "1. Checking cluster status..."
curl -sf http://localhost:8080/api/v1/cluster/status | jq .

echo "2. Uploading a test file..."
dd if=/dev/urandom of=/tmp/test-10mb.bin bs=1M count=10 status=none
UPLOAD_RESP=$(curl -sf -X POST http://localhost:8080/api/v1/upload \
  -H "X-API-Key: demo-key" \
  -F "file=@/tmp/test-10mb.bin")
FILE_ID=$(echo "$UPLOAD_RESP" | jq -r '.file_id')
echo "Uploaded: $FILE_ID"

echo "3. Downloading the file and verifying integrity..."
curl -sf "http://localhost:8080/api/v1/files/${FILE_ID}/download" \
  -H "X-API-Key: demo-key" -o /tmp/downloaded.bin
diff /tmp/test-10mb.bin /tmp/downloaded.bin && echo "✓ File integrity verified"

echo "4. Simulating node failure..."
docker stop dfs-storage-3 || true
sleep 20
curl -sf http://localhost:8080/api/v1/cluster/status | jq '.nodes[] | {id: .NodeID, status: .Status}'

echo "5. Downloading again after node failure (served from replicas)..."
curl -sf "http://localhost:8080/api/v1/files/${FILE_ID}/download" \
  -H "X-API-Key: demo-key" -o /tmp/post-failure.bin
diff /tmp/test-10mb.bin /tmp/post-failure.bin && echo "✓ Download succeeded despite node failure"

echo "6. Viewing metrics..."
curl -sf http://localhost:9091/metrics | grep dfs_ | head -20

echo "=== Demo complete ==="
