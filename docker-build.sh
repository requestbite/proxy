#!/bin/bash
# Helper script to build Docker image with proper version information

set -e

# Extract version information (same logic as Makefile)
if git describe --tags --exact-match 2>/dev/null >/dev/null; then
    VERSION=$(git describe --tags --exact-match | sed 's/^v//')
else
    VERSION=$(git describe --tags 2>/dev/null | sed 's/^v//' | sed 's/-[0-9]\+-g/-/' || echo "dev")
fi

BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Image name (can be overridden with first argument)
IMAGE_NAME="${1:-rb-proxy}"
IMAGE_TAG="${2:-latest}"

echo "Building Docker image with version information:"
echo "  VERSION:    $VERSION"
echo "  BUILD_TIME: $BUILD_TIME"
echo "  GIT_COMMIT: $GIT_COMMIT"
echo "  IMAGE:      $IMAGE_NAME:$IMAGE_TAG"
echo ""

# Build the Docker image with build arguments
docker build \
    --build-arg VERSION="$VERSION" \
    --build-arg BUILD_TIME="$BUILD_TIME" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    -t "$IMAGE_NAME:$IMAGE_TAG" \
    -t "$IMAGE_NAME:$VERSION" \
    .

echo ""
echo "✓ Build complete!"
echo "  Tagged as: $IMAGE_NAME:$IMAGE_TAG"
echo "  Tagged as: $IMAGE_NAME:$VERSION"
echo ""
echo "Run with: docker run -d -p 7331:7331 $IMAGE_NAME:$IMAGE_TAG"
