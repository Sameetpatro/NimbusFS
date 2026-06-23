#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT}/proto"
MODULE="github.com/Sameetpatro/NimbusFS/distributed-storage"

protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${ROOT}" --go_opt=module="${MODULE}" \
  --go-grpc_out="${ROOT}" --go-grpc_opt=module="${MODULE}" \
  "${PROTO_DIR}/storage.proto" \
  "${PROTO_DIR}/master.proto"

echo "proto generation complete"
