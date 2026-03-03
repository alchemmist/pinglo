#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=integrations/lib/pinglo-dot.sh
source "$ROOT_DIR/integrations/lib/pinglo-dot.sh"

PROVIDER="codex"
STATE_DB="${CODEX_STATE_DB:-$HOME/.codex/state_5.sqlite}"
POLL_INTERVAL="${PINGLO_CODEX_POLL_INTERVAL:-0.7}"
COLOR_WAITING="#ffc66d"
COLOR_DONE="#98c379"
COLOR_FAILED="#e06c75"
PINGLO_CODEX_MON_PID=""

sql_quote() {
  local value="$1"
  value=${value//\'/\'\'}
  printf "%s" "$value"
}

status_from_rollout() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    printf "running"
    return
  fi

  local last
  # Interactive Codex sessions emit task lifecycle + final_answer markers.
  last="$(rg -N '"type":"(task_started|task_complete|turn.started|turn.completed|turn.failed|error)"|"phase":"final_answer"' "$path" 2>/dev/null | tail -n 1 || true)"

  if [[ "$last" == *'"type":"turn.failed"'* || "$last" == *'"type":"error"'* ]]; then
    printf "failed"
  elif [[ "$last" == *'"type":"task_complete"'* || "$last" == *'"type":"turn.completed"'* || "$last" == *'"phase":"final_answer"'* ]]; then
    printf "success"
  elif [[ "$last" == *'"type":"task_started"'* || "$last" == *'"type":"turn.started"'* ]]; then
    printf "running"
  else
    printf "running"
  fi
}

tooltip_for() {
  local thread_id="$1"
  local title="$2"
  local status="$3"
  local status_text=""

  case "$status" in
    running) status_text="waiting for response" ;;
    success) status_text="response received" ;;
    failed) status_text="failed" ;;
    *) status_text="$status" ;;
  esac

  title="$(printf '%s' "$title" | tr '\n' ' ' | cut -c1-140)"
  printf 'Codex chat: %s\nStatus: %s\n%s' "$thread_id" "$status_text" "$title"
}

set_thread_dot() {
  local thread_id="$1"
  local title="$2"
  local status="$3"
  local color="$COLOR_WAITING"

  case "$status" in
    success) color="$COLOR_DONE" ;;
    failed) color="$COLOR_FAILED" ;;
  esac

  pinglo_integration_set "$PROVIDER" "$thread_id" "$status" "$(tooltip_for "$thread_id" "$title" "$status")" "$color"
}

sync_threads_once() {
  local start_epoch="$1"
  local cwd="$2"
  local cwd_q
  cwd_q="$(sql_quote "$cwd")"

  if [[ ! -f "$STATE_DB" ]]; then
    return
  fi

  local q
  q="SELECT id, COALESCE(title, ''), rollout_path FROM threads WHERE source='cli' AND archived=0 AND cwd='${cwd_q}' AND updated_at >= ${start_epoch} ORDER BY updated_at DESC LIMIT 64;"

  while IFS=$'\t' read -r thread_id title rollout_path; do
    [[ -n "${thread_id:-}" ]] || continue
    local status
    status="$(status_from_rollout "$rollout_path")"
    set_thread_dot "$thread_id" "$title" "$status"
  done < <(sqlite3 -noheader -separator $'\t' "$STATE_DB" "$q" 2>/dev/null || true)
}

run_with_monitor() {
  if ! command -v codex >/dev/null 2>&1; then
    echo "codex integration error: codex binary not found" >&2
    return 1
  fi
  if ! command -v rg >/dev/null 2>&1; then
    echo "codex integration error: required CLI 'rg' not found" >&2
    return 1
  fi
  if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "codex integration error: required CLI 'sqlite3' not found" >&2
    return 1
  fi
  if [[ ! -f "$STATE_DB" ]]; then
    echo "codex integration error: state db not found at $STATE_DB" >&2
    return 1
  fi

  cleanup_monitor() {
    local pid="${PINGLO_CODEX_MON_PID:-}"
    if [[ -z "$pid" ]]; then
      return
    fi
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
    wait "$pid" 2>/dev/null || true
    PINGLO_CODEX_MON_PID=""
  }

  local start_epoch cwd mon_pid codex_ec
  start_epoch="$(date +%s)"
  cwd="$PWD"

  (
    while true; do
      sync_threads_once "$start_epoch" "$cwd"
      sleep "$POLL_INTERVAL"
    done
  ) &
  mon_pid=$!
  PINGLO_CODEX_MON_PID="$mon_pid"
  trap cleanup_monitor EXIT INT TERM

  set +e
  codex "$@"
  codex_ec=$?
  set -e

  trap - EXIT INT TERM
  cleanup_monitor

  sync_threads_once "$start_epoch" "$cwd"
  return "$codex_ec"
}

usage() {
  cat <<'USAGE'
Codex TUI with pinglo integration

Usage:
  integrations/codex/codex-with-pinglo.sh [codex-args...]

Examples:
  integrations/codex/codex-with-pinglo.sh
  integrations/codex/codex-with-pinglo.sh --profile work
  integrations/codex/codex-with-pinglo.sh --search
USAGE
}

main() {
  if [[ "${1:-}" == "help" || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    return 0
  fi
  run_with_monitor "$@"
}

main "$@"
