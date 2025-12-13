#!/bin/bash
set -euo pipefail

# Environment variables expected:
# A_HOST, A_USER, A_TOKEN, B_HOST, B_USER, B_TOKEN, DRY_RUN

if [ -z "${A_HOST:-}" ] || [ -z "${A_USER:-}" ] || [ -z "${A_TOKEN:-}" ] || [ -z "${B_HOST:-}" ] || [ -z "${B_USER:-}" ] || [ -z "${B_TOKEN:-}" ]; then
  echo "One or more required environment variables are missing: A_HOST A_USER A_TOKEN B_HOST B_USER B_TOKEN"
  exit 1
fi

DRY_FLAG="-dry-run=true"
if [ "${DRY_RUN:-true}" = "false" ]; then
  DRY_FLAG="-dry-run=false"
fi

exec /usr/local/bin/syncplayed \
  -a-host "${A_HOST}" -a-user "${A_USER}" -a-token "${A_TOKEN}" \
  -b-host "${B_HOST}" -b-user "${B_USER}" -b-token "${B_TOKEN}" \
  ${DRY_FLAG}
