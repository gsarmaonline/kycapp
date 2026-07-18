#!/bin/sh
set -eu

# Compose default: api:8080. Railway: e.g. api.railway.internal:${API_PORT}
API_UPSTREAM="${API_UPSTREAM:-api:8080}"
# Railway injects PORT; local Docker image listens on 80.
LISTEN_PORT="${PORT:-80}"

sed -e "s|__API_UPSTREAM__|${API_UPSTREAM}|g" \
    -e "s|__LISTEN_PORT__|${LISTEN_PORT}|g" \
    /etc/nginx/nginx.conf.template > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
