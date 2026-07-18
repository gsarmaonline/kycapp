#!/bin/sh
set -e
# Railway volumes are often root-owned; the API runs as uid 10001 (app).
UPLOAD_DIR="${UPLOAD_DIR:-/data/uploads}"
mkdir -p "$UPLOAD_DIR"
if ! chown -R app:app "$UPLOAD_DIR" 2>/dev/null; then
  echo "warn: could not chown $UPLOAD_DIR (logo uploads may fail)" >&2
fi
exec su-exec app /app/api
