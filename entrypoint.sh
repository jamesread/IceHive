#!/bin/sh

set -e

case "$ICEHIVE_SERVICE" in
frontend)
    if [ -n "${ICEHIVE_FRONTEND_PORT:-}" ]; then
        FE_PORT="${ICEHIVE_FRONTEND_PORT}"
    elif [ -n "${PORT:-}" ]; then
        # Kubernetes sets PORT to tcp://<ip>:<port>; take the port after the last colon.
        FE_PORT="${PORT##*:}"
    else
        FE_PORT=8080
    fi
    FE_PORT=$(printf '%s' "$FE_PORT" | tr -cd '0-9')
    : "${FE_PORT:=8080}"
    sed "s/@LISTEN_PORT@/${FE_PORT}/g" /etc/nginx/icehive-frontend.conf.template >/tmp/icehive-frontend-nginx.conf
    exec nginx -c /tmp/icehive-frontend-nginx.conf -g 'daemon off;'
    ;;
*)
    exec "/usr/local/bin/${ICEHIVE_SERVICE}" "$@"
    ;;
esac
