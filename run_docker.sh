#!/bin/bash

# Remove the existing container if it exists
docker rm -f openviking 2>/dev/null || true

# Run the container with absolute volume paths
docker run -d \
  --name openviking \
  -p 1933:1933 \
  -v "$(pwd)/.openviking/ov.conf:/app/ov.conf" \
  -v "$(pwd)/data:/app/data" \
  --restart unless-stopped \
  ghcr.io/volcengine/openviking:main

echo "OpenViking container started successfully."
