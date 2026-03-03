#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=integrations/lib/pinglo-dot.sh
source "$ROOT_DIR/integrations/lib/pinglo-dot.sh"

PROVIDER="template"
ENTITY="example"

pinglo_integration_set "$PROVIDER" "$ENTITY" running "template integration: working"
sleep 1
pinglo_integration_set "$PROVIDER" "$ENTITY" success "template integration: done"
