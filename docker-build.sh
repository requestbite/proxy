#!/bin/bash
# Helper script to build Docker image with proper version information

set -e

# Show usage if no parameters provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <image-name> [version]"
    echo ""
    echo "Arguments:"
    echo "  image-name    Docker image name (e.g., requestbite/requestbite-proxy)"
    echo "  version       Optional version tag (default: auto-detect from git)"
    echo ""
    echo "Examples:"
    echo "  $0 requestbite/requestbite-proxy"
    echo "  $0 requestbite/requestbite-proxy 0.4.1"
    exit 1
fi

IMAGE_NAME="$1"

# Extract version information
if [ -n "$2" ]; then
    # Use explicitly provided version
    VERSION="$2"
elif git describe --tags --exact-match 2>/dev/null >/dev/null; then
    # We're exactly on a tag
    VERSION=$(git describe --tags --exact-match | sed 's/^v//')
else
    # Get the latest tag without commit info
    VERSION=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
fi

BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

IMAGE_TAG="latest"

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
