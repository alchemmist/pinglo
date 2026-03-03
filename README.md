<h1><img src="./assets/logo.png" alt="Favicon Preview" width="90" align="center"> pinglo</h1>

Minimal status tracker for `waybar` written in Go.

`pinglod` manages dots in memory and persists them to disk, while `pinglo` sends events (`start`, `done`, `clear`) and renders the JSON payload for a Waybar module.

## What’s implemented

- Yellow dot when a command becomes `running` (`start`)
- Green dot when the command finishes successfully (`done --exit-code 0`)
- Red dot when the command finishes with a failure (`done --exit-code != 0`)
- Simultaneous tracking of long-running commands
- Deduplication by `cwd + command`: rerunning the same command in the same directory updates the same dot
- Clearing all dots (`clear`)

## Build

```bash
go build -o pinglod ./cmd/pinglod
go build -o pinglo ./cmd/pinglo
```

## Run the daemon

```bash
pinglod
```

`pinglod` automatically persists dots to a state file so that your indicators survive restarts. The location is chosen in order:

- `$PINGLO_STATE_FILE`, if set
- otherwise `$XDG_DATA_HOME/pinglo/state.json` (defaults to `~/.local/share/pinglo/state.json`)
- otherwise the system temp dir.

You can override the path to keep state on a shared drive or use a RAM-backed file.

`pinglod` will notify any Waybar processes with `SIGRTMIN+4` (or the offset specified via `-signal-offset`) after every `start`, `done`, or `clear`, so the module refreshes only when the state actually changes.

Default socket selection:

- `$PINGLO_SOCKET`, if set
- otherwise `$XDG_RUNTIME_DIR/pinglo.sock`
- otherwise `/tmp/pinglo-<uid>.sock`

## CLI commands

```bash
# mark a dot as running
pinglo start --cmd "sleep 10" --cwd "$PWD"

# finish the same dot
pinglo done --cmd "sleep 10" --cwd "$PWD" --exit-code 0

# clear the module
pinglo clear

# inspect the current state
pinglo list
```

## Generic dot API

You can manage labeled dots with colors and tooltips without tying them to shell commands.

```bash
# add/update a dot
pinglo dot set --id deploy --color "#ffc66d" --tooltip "Deploy running" --status running

# mark it complete
pinglo dot set --id deploy --status success --tooltip "Deploy succeeded"

# remove a dot
pinglo dot remove --id deploy
```

`dot set` accepts `--status` values `running`, `success`, or `failed` (defaults to `running` if omitted). The tooltip text is displayed as supplied, and the color is applied directly to the dot in Waybar. Use this API for any indicator that only needs a colored point and a short tooltip; the classic `start`/`done` helpers keep working for command tracking.

Because the daemon persists its state, dots created via `dot set` survive `pinglod` restarts and even system boot, so long as the state file remains reachable.

## Integrations architecture

`pinglo` is intentionally split into:

- Small core daemon + CLI (`pinglod`, `pinglo`)
- Generic `dot` API for external producers
- Provider-specific integrations outside the core

This lets you add new sources of status (tasks, Codex chats, CI pipelines, deploy hooks, etc.) without growing `internal/pinglo` and without changing the wire protocol.

### Why keep integrations outside the core

- The core stays stable: only state management and rendering logic live there.
- Integration-specific lifecycle rules remain isolated.
- You can ship, test, and version integrations independently.
- New providers can be prototyped as scripts before promoting to dedicated binaries.

### Integration filesystem layout

```text
integrations/
  README.md
  lib/
    pinglo-dot.sh
  templates/
    integration-template.sh
  codex/
    ...
```

### Integration contract

Every integration should only use these public commands:

- `pinglo dot set --id ... --status running|success|failed --tooltip ... [--color ...]`
- `pinglo dot remove --id ...`

No integration should write state files directly or call daemon internals.

### Dot ID namespace

Use namespaced IDs so integration dots never collide with task dots or manual dots:

`integration:<provider>:<entity-id>`

Examples:

- `integration:codex:019cb508-f0e6-7201-86d1-0ece0e906456`
- `integration:ci:build-7841`
- `integration:deploy:prod-eu-west-1`

### Lifecycle recommendation

Use a simple state machine per external entity:

1. Create/update dot as `running` when work starts.
2. Update dot to `success` when completed.
3. Update dot to `failed` if an error occurs.
4. Optionally remove the dot when entity is no longer relevant.

Keep tooltip concise and actionable; first line should identify source and entity, next lines can include status details.

### Reusable helper library

`integrations/lib/pinglo-dot.sh` contains shared helpers:

- `pinglo_integration_dot_id <provider> <entity>`
- `pinglo_integration_set <provider> <entity> <status> <tooltip> [color]`
- `pinglo_integration_remove <provider> <entity>`

Integrations should source this file instead of duplicating shell glue.

### Create a new integration

1. Copy template:

```bash
cp integrations/templates/integration-template.sh integrations/<provider>/<provider>-watch.sh
chmod +x integrations/<provider>/<provider>-watch.sh
```

2. Replace:
- `PROVIDER` with provider name (e.g. `codex`, `ci`, `k8s`)
- `ENTITY` with a stable per-item identifier (task id, thread id, build id, etc.)
- status updates with your provider event mapping

3. Run integration side-by-side with `pinglod`; dots will appear in existing Waybar module immediately.

### Event-to-status mapping guideline

Recommended default mapping:

- in-progress/waiting -> `running`
- done/ok -> `success`
- error/cancel/timeout -> `failed`

Override color only when provider semantics require custom palette; otherwise rely on default status colors.

## Codex CLI integration

`pinglo` includes two external Codex wrappers:

- `integrations/codex/codex-with-pinglo.sh` for interactive TUI (`codex`)
- `integrations/codex/pinglo-codex-chat.sh` for non-interactive `codex exec --json`

Use plain `codex` if you do not want tracking. Use `codex-with-pinglo.sh` when you want Waybar dots.

### What it does

- Creates a dot in `running` (yellow) while waiting for the model response.
- Switches dot to `success` (green) when response is completed.
- Switches dot to `failed` (red) on stream failure or non-zero Codex exit.
- Uses thread-aware IDs (`integration:codex:<thread_id>`) so each chat is rendered as a separate dot.

### Run interactive Codex with tracking

```bash
# start interactive TUI + pinglo tracking
integrations/codex/codex-with-pinglo.sh

# same codex args are forwarded
integrations/codex/codex-with-pinglo.sh --profile work --search
```

Optional convenience target:

```bash
make run-codex-integration
```

`codex-with-pinglo.sh` reads Codex local state (`~/.codex/state_5.sqlite`) and each thread `rollout_path`, then maps latest events to dot status.

### Optional non-interactive mode

```bash
integrations/codex/pinglo-codex-chat.sh exec "summarize current repo"
```

Both wrappers use thread-aware IDs (`integration:codex:<thread_id>`) so each Codex chat is rendered as a separate dot.

## Waybar: config snippet

Add this module definition to your `~/.config/waybar/config`. `pinglod` uses `SIGRTMIN+4` by default, so the module must watch `signal: 4` and refresh `interval: "once"`:

```json
{
  "modules-right": ["custom/pinglo"],
  "custom/pinglo": {
    "return-type": "json",
    "exec": "pinglo render --format waybar",
    "interval": "once",
    "signal": 4,
    "escape": false,
    "tooltip": true,
    "markup": "pango",
    "on-click": "pinglo clear"
  }
}
```

If you need a different real-time signal, start `pinglod` with `-signal-offset N` and set the module `signal` to the same offset.

## Waybar: style snippet

In `~/.config/waybar/style.css`:

```css
#custom-pinglo {
  font-size: 14px;
  border-radius: 10px;
  margin-right: 5px;
}

#custom-pinglo.empty {
  padding: 0;
  margin: 0;
}

#custom-pinglo.running {
  color: #e5c07b;
}

#custom-pinglo.success {
  color: #98c379;
}

#custom-pinglo.failed {
  color: #e06c75;
}
```

Waybar receives dots as a Pango-marked string (`text`), so each dot is rendered with a `<span foreground="…">●</span>` and CSS cannot target those spans by `class`. The `class` field therefore remains semantic (one of `running`, `success`, `failed`, `empty`) so you can color the module container via selectors like `#custom-pinglo.running`. The order of dots still matches the order commands were started, and the tooltip lists them in the same sequence.

## Basic shell flow

Manual flow:

```bash
pinglo start --cmd "long-command" --cwd "$PWD"
long-command
pinglo done --cmd "long-command" --cwd "$PWD" --exit-code $?
```

### Zsh hook for commands prefixed with a space

Add to your `~/.zshrc`:

```zsh
autoload -Uz add-zsh-hook

function _pinglo_preexec() {
  local raw="$1"

  if [[ "$raw" == ' '* ]]; then
    export PINGLO_TRACKED_CMD="${raw# }"
    pinglo start --cmd "$PINGLO_TRACKED_CMD" --cwd "$PWD" >/dev/null 2>&1
  else
    unset PINGLO_TRACKED_CMD
  fi
}

function _pinglo_precmd() {
  local exit_code=$?
  if [[ -n "$PINGLO_TRACKED_CMD" ]]; then
    pinglo done --cmd "$PINGLO_TRACKED_CMD" --cwd "$PWD" --exit-code "$exit_code" >/dev/null 2>&1
    unset PINGLO_TRACKED_CMD
  fi
}

add-zsh-hook preexec _pinglo_preexec
add-zsh-hook precmd _pinglo_precmd
```

Call `pinglo clear` to wipe the module manually.

## Limitations

- The shell hook example tracks a single active command per session; extending it to multiple concurrent commands requires additional bookkeeping.
