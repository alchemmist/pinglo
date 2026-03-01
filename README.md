# pinglo

Minimal AnyBar-like indicator for `waybar` written in Go.

`pinglod` keeps the in-memory state of dots, while `pinglo` sends events (`start`, `done`, `clear`) and renders the JSON payload for a Waybar module.

## What’s implemented

- Yellow dot when a command becomes `running` (`start`)
- Green dot when the command finishes successfully (`done --exit-code 0`)
- Red dot when the command finishes with a failure (`done --exit-code != 0`)
- Simultaneous tracking of up to ten long-running commands
- Deduplication by `cwd + command`: rerunning the same command in the same directory updates the same dot
- Clearing all dots (`clear`)

## Build

```bash
go build -o ./bin/pinglod ./cmd/pinglod
go build -o ./bin/pinglo ./cmd/pinglo
```

## Run the daemon

```bash
./bin/pinglod
```

Default socket selection:

- `$PINGLO_SOCKET`, if set
- otherwise `$XDG_RUNTIME_DIR/pinglo.sock`
- otherwise `/tmp/pinglo-<uid>.sock`

## CLI commands

```bash
# mark a dot as running
./bin/pinglo start --cmd "sleep 10" --cwd "$PWD"

# finish the same dot
./bin/pinglo done --cmd "sleep 10" --cwd "$PWD" --exit-code 0

# clear the module
./bin/pinglo clear

# inspect the current state
./bin/pinglo list
```

## Waybar: config snippet

Add this module definition to your `~/.config/waybar/config`:

```json
{
  "modules-right": ["custom/pinglo"],
  "custom/pinglo": {
    "return-type": "json",
    "exec": "./bin/pinglo render --format waybar",
    "interval": 1,
    "escape": false,
    "tooltip": true
  }
}
```

If `modules-right` already exists, append `"custom/pinglo"` to the array.

## Waybar: style snippet

In `~/.config/waybar/style.css`:

```css
#custom-pinglo {
  padding: 0 8px;
  margin: 0 6px;
  font-size: 14px;
}

#custom-pinglo.empty {
  padding: 0;
  margin: 0;
}
```

Dot colors are encoded in the Pango markup emitted by `pinglo render`:

- running → `#e5c07b`
- success → `#98c379`
- failed → `#e06c75`

## Basic shell flow

Manual flow:

```bash
./bin/pinglo start --cmd "long-command" --cwd "$PWD"
long-command
./bin/pinglo done --cmd "long-command" --cwd "$PWD" --exit-code $?
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

- State is held in memory; restarting the daemon clears everything.
- The shell hook example tracks a single active command per session; extending it to multiple concurrent commands requires additional bookkeeping.
