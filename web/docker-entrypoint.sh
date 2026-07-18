#!/bin/sh
set -eu

# Compose default: api:8080. Railway: e.g. kycapp.railway.internal:8080
API_UPSTREAM="${API_UPSTREAM:-api:8080}"
# Railway injects PORT; local Docker image listens on 80.
LISTEN_PORT="${PORT:-80}"

# Docker embedded DNS vs Railway private DNS.
case "$API_UPSTREAM" in
  *railway.internal*)
    # Brackets required for IPv6 in nginx resolver directives.
    NGINX_RESOLVER="${NGINX_RESOLVER:-[fd12::10]}"
    ;;
  *)
    NGINX_RESOLVER="${NGINX_RESOLVER:-127.0.0.11}"
    ;;
esac

sed -e "s|__API_UPSTREAM__|${API_UPSTREAM}|g" \
    -e "s|__LISTEN_PORT__|${LISTEN_PORT}|g" \
    -e "s|__NGINX_RESOLVER__|${NGINX_RESOLVER}|g" \
    /etc/nginx/nginx.conf.template > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
