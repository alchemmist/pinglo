#!/usr/bin/env bash
set -euo pipefail

# Shared helpers for external pinglo integrations.

PINGLO_BIN="${PINGLO_BIN:-pinglo}"
PINGLO_INTEGRATION_PREFIX="${PINGLO_INTEGRATION_PREFIX:-integration}"

_pinglo_require_bin() {
	if ! command -v "$PINGLO_BIN" >/dev/null 2>&1; then
		echo "pinglo integration error: binary '$PINGLO_BIN' not found" >&2
		return 1
	fi
}

_pinglo_run() {
	_pinglo_require_bin
	"$PINGLO_BIN" "$@"
}

pinglo_integration_dot_id() {
	local provider="$1"
	local entity="$2"

	provider="${provider// /-}"
	entity="${entity// /-}"
	printf '%s:%s:%s' "$PINGLO_INTEGRATION_PREFIX" "$provider" "$entity"
}

pinglo_integration_set() {
	local provider="$1"
	local entity="$2"
	local status="$3"
	local tooltip="$4"
	local color="${5:-}"

	local dot_id
	dot_id="$(pinglo_integration_dot_id "$provider" "$entity")"

	if [[ -n "$color" ]]; then
		_pinglo_run dot set --id "$dot_id" --status "$status" --tooltip "$tooltip" --color "$color" >/dev/null
	else
		_pinglo_run dot set --id "$dot_id" --status "$status" --tooltip "$tooltip" >/dev/null
	fi
}

pinglo_integration_remove() {
	local provider="$1"
	local entity="$2"
	local dot_id
	dot_id="$(pinglo_integration_dot_id "$provider" "$entity")"
	_pinglo_run dot remove --id "$dot_id" >/dev/null || true
}
