#!/usr/bin/env bash
# Seeds the throwaway controller with fictional config and collection sources
# via Connect RPC (JSON). Usage: seed.sh <controller-base-url>
set -euo pipefail

base="${1:?controller base url}"
here="$(cd "$(dirname "$0")" && pwd)"
api="$base/api/icehive.v1.ControllerService"

rpc() {
	curl -fsS -X POST -H 'Content-Type: application/json' -d "$2" "$api/$1" >/dev/null
}

jq -c 'to_entries[] | {key, value}' "$here/seed/config.json" | while read -r kv; do
	rpc SetConfig "$kv"
done

jq -c '.[]' "$here/seed/sources.json" | while read -r src; do
	rpc UpsertCollectionSource "{\"source\": $src}"
done
