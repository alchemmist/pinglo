# pinglo

Минималистичный AnyBar-подобный индикатор для `waybar` на Go.

`pinglod` хранит состояние точек (in-memory), `pinglo` отправляет события (`start`, `done`, `clear`) и рендерит модуль для `waybar`.

## Что уже реализовано

- Желтая точка для команды в работе (`start`)
- Зеленая точка при успешном завершении (`done --exit-code 0`)
- Красная точка при ошибке (`done --exit-code != 0`)
- Несколько параллельных задач одновременно
- Дедупликация: одинаковая `command` в той же `cwd` обновляет ту же точку
- Очистка всех точек (`clear`)

## Сборка

```bash
go build -o ./bin/pinglod ./cmd/pinglod
go build -o ./bin/pinglo ./cmd/pinglo
```

## Запуск демона

```bash
./bin/pinglod
```

Сокет по умолчанию:

- `$PINGLO_SOCKET`, если задан
- иначе `$XDG_RUNTIME_DIR/pinglo.sock`
- иначе `/tmp/pinglo-<uid>.sock`

## Команды CLI

```bash
# создать/обновить точку как running
./bin/pinglo start --cmd "sleep 10" --cwd "$PWD"

# завершить точку
./bin/pinglo done --cmd "sleep 10" --cwd "$PWD" --exit-code 0

# очистить все точки
./bin/pinglo clear

# посмотреть текущее состояние
./bin/pinglo list
```

## Waybar: модуль в config

Добавь в `~/.config/waybar/config`:

```json
{
  "modules-right": ["custom/pinglo"],
  "custom/pinglo": {
    "return-type": "json",
    "exec": "/home/alchemmist/code/pinglo/bin/pinglo render --format waybar",
    "interval": 1,
    "escape": false,
    "tooltip": true
  }
}
```

Если у тебя уже есть `modules-right`, просто добавь туда `"custom/pinglo"`.

## Waybar: стили в style.css

Добавь в `~/.config/waybar/style.css`:

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

Цвета точек задаются самим `pinglo render` через Pango-разметку:

- running: `#e5c07b` (желтый)
- success: `#98c379` (зеленый)
- failed: `#e06c75` (красный)

## Базовый shell flow

Минимально вручную:

```bash
./bin/pinglo start --cmd "long-command" --cwd "$PWD"
long-command
./bin/pinglo done --cmd "long-command" --cwd "$PWD" --exit-code $?
```

### Zsh-хуки (базовый автотрекинг команд с ведущим пробелом)

Добавь в `~/.zshrc`:

```zsh
autoload -Uz add-zsh-hook

function _pinglo_preexec() {
  local raw="$1"

  # Отслеживаем только команды, которые введены с ведущим пробелом.
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

Для очистки модуля:

```bash
pinglo clear
```

## Ограничения базовой версии

- Состояние хранится в памяти демона и сбрасывается после его перезапуска.
- В одном shell одновременный запуск нескольких фоновых команд через хуки может потребовать более сложного трекинга (в базовой версии хранится один активный `PINGLO_TRACKED_CMD` на сессию).
