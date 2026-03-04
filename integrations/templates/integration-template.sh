#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=integrations/lib/pinglo-dot.sh
source "$ROOT_DIR/integrations/lib/pinglo-dot.sh"

PROVIDER="template"
ENTITY="example-$(date +%s)-$$"

on_exit() {
	if [[ "$?" -ne 0 ]]; then
		pinglo_integration_set "$PROVIDER" "$ENTITY" failed "template integration: failed"
	fi
}
trap on_exit EXIT

pinglo_integration_set "$PROVIDER" "$ENTITY" running "template integration: working"
sleep 1
pinglo_integration_set "$PROVIDER" "$ENTITY" success "template integration: done"
trap - EXIT
