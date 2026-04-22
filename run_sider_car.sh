#!/bin/bash

# Remove the existing container if it exists
docker rm -f openvk-grep-sidecar 2>/dev/null || true

# Pull the latest image
docker pull enzii/openvk-grep-sidecar:latest

# Run the grep-sidecar container alongside the OpenViking container
docker run -d \
  --name openvk-grep-sidecar \
  -p 1935:1935 \
  -v "$(pwd)/data/workspace:/data/workspace:ro" \
  -e GREP_PORT=1935 \
  -e GREP_TIMEOUT=30s \
  -e GREP_MAX_RESULTS=500 \
  -e GREP_MAX_FILESIZE=50M \
  -e OPEN_VIKING_DATA_PATH=/data/workspace/viking \
  -e OPEN_VIKING_ACCOUNT=default \
  --restart unless-stopped \
  enzii/openvk-grep-sidecar:latest

echo "grep-sidecar container started successfully on port 1935."
