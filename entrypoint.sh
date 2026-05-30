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

# ICEHIVE_CONTROLLER_PORT is injected when the Pod links to the controller Service
# (e.g. tcp://10.96.79.247:8080). Falls back to *_SERVICE_HOST / *_SERVICE_PORT, then localhost.
resolve_controller_upstream() {
    if [ -n "${ICEHIVE_CONTROLLER_PORT:-}" ]; then
        val="$ICEHIVE_CONTROLLER_PORT"
        case "$val" in
        tcp://*)
            CONTROLLER_UPSTREAM="http://${val#tcp://}"
            CONTROLLER_UPSTREAM_FROM="ICEHIVE_CONTROLLER_PORT"
            return
            ;;
        http://*|https://*)
            CONTROLLER_UPSTREAM="$val"
            CONTROLLER_UPSTREAM_FROM="ICEHIVE_CONTROLLER_PORT"
            return
            ;;
        esac
    fi
    if [ -n "${ICEHIVE_CONTROLLER_SERVICE_HOST:-}" ]; then
        port=$(parse_listen_port "${ICEHIVE_CONTROLLER_SERVICE_PORT:-8080}")
        CONTROLLER_UPSTREAM="http://${ICEHIVE_CONTROLLER_SERVICE_HOST}:${port}"
        CONTROLLER_UPSTREAM_FROM="ICEHIVE_CONTROLLER_SERVICE_HOST/ICEHIVE_CONTROLLER_SERVICE_PORT"
        return
    fi
    CONTROLLER_UPSTREAM="http://127.0.0.1:8080"
    CONTROLLER_UPSTREAM_FROM="default (http://127.0.0.1:8080)"
}

# persister-yaml commits YAML snapshots; git requires a configured identity in containers.
configure_git_identity() {
    git_email="${GIT_USER_EMAIL:-persister-yaml@icehive.local}"
    git_name="${GIT_USER_NAME:-IceHive persister-yaml}"
    git config --global user.email "$git_email"
    git config --global user.name "$git_name"
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
    resolve_controller_upstream
    printf '%s\n' "controller upstream ${CONTROLLER_UPSTREAM} (from ${CONTROLLER_UPSTREAM_FROM})"
    sed -e "s/@LISTEN_PORT@/${FE_PORT}/g" \
        -e "s|@CONTROLLER_UPSTREAM@|${CONTROLLER_UPSTREAM}|g" \
        /etc/nginx/icehive-frontend.conf.template >/tmp/icehive-frontend-nginx.conf
    exec nginx -c /tmp/icehive-frontend-nginx.conf -g 'daemon off;'
    ;;
persister-yaml)
    configure_git_identity
    exec "/usr/local/bin/persister-yaml" "$@"
    ;;
*)
    exec "/usr/local/bin/${ICEHIVE_SERVICE}" "$@"
    ;;
esac
