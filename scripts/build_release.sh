#!/usr/bin/env bash
set -euo pipefail

DIST_DIR="${DIST_DIR:-dist}"
CMD_PATH="./cmd/termchat"
APP_NAME="termchat"
PLATFORMS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

for target in "${PLATFORMS[@]}"; do
  IFS="/" read -r goos goarch <<<"${target}"
  out="${APP_NAME}-${goos}-${goarch}"
  if [[ "${goos}" == "windows" ]]; then
    out="${out}.exe"
  fi
  echo "building ${out}"
  GOOS="${goos}" GOARCH="${goarch}" go build -o "${DIST_DIR}/${out}" "${CMD_PATH}"
done

echo "artifacts written to ${DIST_DIR}/"

