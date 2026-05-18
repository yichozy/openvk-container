#!/bin/bash

# Remove the existing container if it exists
docker rm -f openvk-grep-sidecar 2>/dev/null || true

# Pull the latest image
docker pull enzii/ov-sidecar:latest

# Run the grep-sidecar container alongside the OpenViking container
docker run -d \
  --name openvk-grep-sidecar \
  -p 1935:1935 \
  -v "$(pwd)/data/workspace:/data/workspace:ro" \
  -e SIDECAR_PORT=1935 \
  -e SIDECAR_TIMEOUT=30s \
  -e MAX_GREP_RESULTS=500 \
  -e MAX_GREP_FILESIZE=50M \
  -e SIDECAR_GREP_THREADS="${SIDECAR_GREP_THREADS:-2}" \
  -e RG_THREADS="${SIDECAR_GREP_THREADS:-2}" \
  -e SIDECAR_MAX_CONCURRENCY="${SIDECAR_MAX_CONCURRENCY:-2}" \
  -e OPEN_VIKING_DATA_PATH=/data/workspace/viking \
  -e OPEN_VIKING_ACCOUNT=default \
  --restart unless-stopped \
  enzii/ov-sidecar:latest

echo "grep-sidecar container started successfully on port 1935."
