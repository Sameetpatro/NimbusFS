#!/usr/bin/env bash
# generate_proto.sh runs protoc with consistent flags so make proto is reproducible across machines
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT}/proto"
OUT_DIR="${ROOT}/proto/gen"

# ensure output tree exists before protoc writes nested go_package paths
mkdir -p "${OUT_DIR}"

protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/storage.proto" \
  "${PROTO_DIR}/master.proto"

echo "proto generation complete: ${OUT_DIR}"
