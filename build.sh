#!/bin/bash

# Stop on error
set -e

# Default tag is latest, but can be overridden by passing an argument
TAG=${1:-latest}
IMAGE_NAME="enzii/openvk-container"
FULL_IMAGE_NAME="${IMAGE_NAME}:${TAG}"

echo "🚀 Building Docker image ${FULL_IMAGE_NAME}..."
docker build -t ${FULL_IMAGE_NAME} .

# echo "📤 Pushing Docker image ${FULL_IMAGE_NAME} to Docker Hub..."
# docker push ${FULL_IMAGE_NAME}

echo "✅ Done!"
