#!/usr/bin/env bash
set -euo pipefail

dockerfiles=(Dockerfile backend/Dockerfile frontend/Dockerfile deploy/combined.Dockerfile)
for file in "${dockerfiles[@]}"; do
  while IFS= read -r instruction; do
    if [[ "$instruction" != *"@sha256:"* ]]; then
      echo "Unpinned base image in $file: $instruction" >&2
      exit 1
    fi
  done < <(awk 'toupper($1) == "FROM" { print }' "$file")
done

while IFS= read -r image; do
  if [[ "$image" != *"@sha256:"* ]]; then
    echo "Unpinned Compose image: $image" >&2
    exit 1
  fi
done < <(awk '$1 == "image:" { print $2 }' docker-compose.yml)

echo "PASS: all Dockerfile base images and Compose service images are digest-pinned."
