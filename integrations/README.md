# Integrations

External integrations live here and interact with pinglo through `pinglo dot set/remove`.

## Layout

- `lib/pinglo-dot.sh` shared helper functions for setting/removing namespaced dots
- `templates/integration-template.sh` minimal integration template
- `codex/` provider-specific scripts for Codex CLI integration

## Naming convention

All integration dots use namespaced IDs:

`integration:<provider>:<entity-id>`

Example:

`integration:codex:019cb508-f0e6-7201-86d1-0ece0e906456`

This avoids collisions with task dots and user-defined generic dots.
