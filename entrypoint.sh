#!/bin/sh

printf '%s\n' "IceHive container version: ${ICEHIVE_VERSION:-dev}"

set -e

# Kubernetes may inject PORT (or a copied ICEHIVE_FRONTEND_PORT) as tcp://<ip>:<port>.
parse_listen_port() {
    val="$1"
    case "$val" in
    tcp://*)
        val="${val##*:}"
        ;;
    esac
    val=$(printf '%s' "$val" | tr -cd '0-9')
    if [ -z "$val" ] || [ "$val" -lt 1 ] 2>/dev/null || [ "$val" -gt 65535 ] 2>/dev/null; then
        val=8080
    fi
    printf '%s' "$val"
}

case "$ICEHIVE_SERVICE" in
frontend)
    if [ -n "${ICEHIVE_FRONTEND_PORT:-}" ]; then
        FE_PORT=$(parse_listen_port "$ICEHIVE_FRONTEND_PORT")
    elif [ -n "${PORT:-}" ]; then
        FE_PORT=$(parse_listen_port "$PORT")
    else
        FE_PORT=8080
    fi
    sed "s/@LISTEN_PORT@/${FE_PORT}/g" /etc/nginx/icehive-frontend.conf.template >/tmp/icehive-frontend-nginx.conf
    exec nginx -c /tmp/icehive-frontend-nginx.conf -g 'daemon off;'
    ;;
*)
    exec "/usr/local/bin/${ICEHIVE_SERVICE}" "$@"
    ;;
esac
