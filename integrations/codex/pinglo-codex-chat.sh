#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=integrations/lib/pinglo-dot.sh
source "$ROOT_DIR/integrations/lib/pinglo-dot.sh"

PROVIDER="codex"
COLOR_WAITING="#ffc66d"
COLOR_DONE="#98c379"
COLOR_FAILED="#e06c75"

CURRENT_ENTITY=""
CURRENT_THREAD_ID=""
CURRENT_STATUS="running"

extract_field() {
  local line="$1"
  local field="$2"
  sed -nE "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"([^\"]*)\".*/\1/p" <<<"$line"
}

tooltip_waiting() {
  local entity="$1"
  printf 'Codex chat: %s\nStatus: waiting for response' "$entity"
}

tooltip_done() {
  local entity="$1"
  printf 'Codex chat: %s\nStatus: response received' "$entity"
}

tooltip_failed() {
  local entity="$1"
  local message="${2:-error}"
  printf 'Codex chat: %s\nStatus: failed\n%s' "$entity" "$message"
}

set_waiting() {
  local entity="$1"
  pinglo_integration_set "$PROVIDER" "$entity" running "$(tooltip_waiting "$entity")" "$COLOR_WAITING"
  CURRENT_STATUS="running"
}

set_done() {
  local entity="$1"
  pinglo_integration_set "$PROVIDER" "$entity" success "$(tooltip_done "$entity")" "$COLOR_DONE"
  CURRENT_STATUS="success"
}

set_failed() {
  local entity="$1"
  local message="${2:-error}"
  pinglo_integration_set "$PROVIDER" "$entity" failed "$(tooltip_failed "$entity" "$message")" "$COLOR_FAILED"
  CURRENT_STATUS="failed"
}

switch_entity_if_needed() {
  local next_entity="$1"
  if [[ -z "$next_entity" || "$next_entity" == "$CURRENT_ENTITY" ]]; then
    return
  fi
  CURRENT_ENTITY="$next_entity"
  set_waiting "$CURRENT_ENTITY"
}

consume_stream() {
  local line event_type thread_id message
  while IFS= read -r line; do
    printf '%s\n' "$line"

    [[ "$line" == "{"* ]] || continue

    event_type="$(extract_field "$line" "type")"
    [[ -n "$event_type" ]] || continue

    thread_id="$(extract_field "$line" "thread_id")"
    if [[ -n "$thread_id" ]]; then
      CURRENT_THREAD_ID="$thread_id"
      switch_entity_if_needed "$thread_id"
    fi

    case "$event_type" in
      thread.started|turn.started)
        set_waiting "$CURRENT_ENTITY"
        ;;
      turn.completed)
        set_done "$CURRENT_ENTITY"
        ;;
      turn.failed)
        message="$(extract_field "$line" "message")"
        if [[ -z "$message" ]]; then
          message="turn failed"
        fi
        set_failed "$CURRENT_ENTITY" "$message"
        ;;
      error)
        message="$(extract_field "$line" "message")"
        if [[ -z "$message" ]]; then
          message="unexpected error event"
        fi
        set_failed "$CURRENT_ENTITY" "$message"
        ;;
    esac
  done
}

run_exec() {
  if ! command -v codex >/dev/null 2>&1; then
    echo "codex integration error: codex binary not found" >&2
    return 1
  fi
  if [[ "$#" -eq 0 ]]; then
    echo "codex integration error: missing prompt/args for codex exec" >&2
    usage >&2
    return 2
  fi

  local tmp_dir="" fifo="" codex_pid codex_ec
  cleanup() {
    if [[ -n "$fifo" && -p "$fifo" ]]; then
      rm -f "$fifo"
    fi
    if [[ -n "$tmp_dir" && -d "$tmp_dir" ]]; then
      rmdir "$tmp_dir" 2>/dev/null || true
    fi
  }
  trap cleanup EXIT INT TERM

  tmp_dir="$(mktemp -d)"
  fifo="$tmp_dir/codex-stream"
  mkfifo "$fifo"

  CURRENT_ENTITY="session-$$"
  set_waiting "$CURRENT_ENTITY"

  codex exec --json "$@" >"$fifo" 2>&1 &
  codex_pid=$!

  consume_stream <"$fifo"

  if wait "$codex_pid"; then
    codex_ec=0
  else
    codex_ec=$?
  fi

  if [[ "$codex_ec" -eq 0 ]]; then
    if [[ "$CURRENT_STATUS" == "running" ]]; then
      set_done "$CURRENT_ENTITY"
    fi
  else
    if [[ "$CURRENT_STATUS" != "failed" ]]; then
      set_failed "$CURRENT_ENTITY" "codex exited with code $codex_ec"
    fi
  fi

  trap - EXIT INT TERM
  cleanup
  return "$codex_ec"
}

usage() {
  cat <<'USAGE'
pinglo codex integration

Usage:
  integrations/codex/pinglo-codex-chat.sh exec [codex-exec-args...]

Examples:
  integrations/codex/pinglo-codex-chat.sh exec "summarize current repo"
  integrations/codex/pinglo-codex-chat.sh exec resume --last "continue"
USAGE
}

main() {
  local cmd="${1:-exec}"
  case "$cmd" in
    exec)
      shift || true
      run_exec "$@"
      ;;
    help|-h|--help)
      usage
      ;;
    *)
      echo "unknown subcommand: $cmd" >&2
      usage >&2
      return 1
      ;;
  esac
}

main "$@"
