#!/bin/bash

# Build script for multi-platform termchat binaries
# Creates binaries for macOS, Linux, and Windows

set -e

VERSION=${1:-"dev"}
OUTPUT_DIR="dist"

echo "🔨 Building termchat v${VERSION} for multiple platforms..."

# Clean previous builds
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# Build configurations: OS/ARCH/OUTPUT_NAME
PLATFORMS=(
    "darwin/amd64/termchat-macos-amd64"
    "darwin/arm64/termchat-macos-arm64"
    "linux/amd64/termchat-linux-amd64"
    "linux/arm64/termchat-linux-arm64"
    "windows/amd64/termchat-windows-amd64.exe"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH OUTPUT_NAME <<< "$PLATFORM"
    
    echo "Building ${OUTPUT_NAME}..."
    
    GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o ${OUTPUT_DIR}/${OUTPUT_NAME} \
        ./cmd/client/main.go
    
    # Calculate SHA256
    if command -v sha256sum &> /dev/null; then
        sha256sum ${OUTPUT_DIR}/${OUTPUT_NAME} > ${OUTPUT_DIR}/${OUTPUT_NAME}.sha256
    elif command -v shasum &> /dev/null; then
        shasum -a 256 ${OUTPUT_DIR}/${OUTPUT_NAME} > ${OUTPUT_DIR}/${OUTPUT_NAME}.sha256
    fi
done

echo "✅ Build complete! Binaries in ${OUTPUT_DIR}/"
ls -lh ${OUTPUT_DIR}/
