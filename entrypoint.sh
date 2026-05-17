#!/bin/sh

set -e

case "$ICEHIVE_SERVICE" in
frontend)
    FE_PORT=$(printf '%s' "${ICEHIVE_FRONTEND_PORT:-${PORT:-8080}}" | tr -cd '0-9')
    : "${FE_PORT:=8080}"
    sed "s/@LISTEN_PORT@/${FE_PORT}/g" /etc/nginx/icehive-frontend.conf.template >/tmp/icehive-frontend-nginx.conf
    exec nginx -c /tmp/icehive-frontend-nginx.conf -g 'daemon off;'
    ;;
*)
    exec "/usr/local/bin/${ICEHIVE_SERVICE}" "$@"
    ;;
esac
