# CLI Tool Specific Requirements

### Command Structure

| Команда | Тип | Описание |
|---------|-----|----------|
| `ralph bridge <story-files>` | One-shot | Конвертация BMad stories → sprint-tasks.md |
| `ralph run` | Long-running | Основной loop с execute → review фазами |

#### `ralph bridge`

```
ralph bridge stories/auth.md stories/crud.md
ralph bridge stories/              # все .md файлы в директории
ralph bridge --merge               # Smart Merge с существующим sprint-tasks.md
```

Output: `sprint-tasks.md` с задачами, AC-derived tests, human gates, служебными задачами.

#### `ralph run`

```
ralph run                          # review после каждой задачи
ralph run --gates                  # human gates включены (по разметке из bridge)
ralph run --gates --every 3        # + checkpoint каждые 3 задачи
ralph run --max-iterations 5       # max 5 попыток на задачу
ralph run --max-turns 50           # max 50 ходов Claude за execute-сессию
ralph run --always-extract         # extraction знаний после каждой итерации (не только failure)
```

### Configuration

Двухуровневая: **config file + CLI flags override**.

**Config file** (`.ralph/config.yaml`, формат YAML) в корне проекта:

**Приоритет:** CLI flags > `.ralph/config.yaml` > embedded defaults

#### Полная таблица параметров

| # | Параметр | CLI flag | Config key | Default | Описание |
|---|----------|----------|------------|---------|----------|
| 1 | Max turns per execute | `--max-turns N` | `max_turns` | 30 | Лимит ходов Claude Code за одну execute-сессию (review не ограничивается) |
| 2 | Max iterations per task | `--max-iterations N` | `max_iterations` | 3 | Попытки execute на задачу до emergency gate |
| 3 | Review frequency | — | — | 1 | Review после каждой задачи (MVP: всегда 1, `--review-every N` — Growth) |
| 4 | Review max iterations | — | `review_max_iterations` | 3 | Max циклов execute→review на задачу |
| 5 | Gates enabled | `--gates` | `gates_enabled` | false | Включить human gates |
| 6 | Gates checkpoint | `--every N` | `gates_checkpoint` | 0 | Checkpoint каждые N задач (0 = off) |
| 7 | Execute model | — | `model` | opus | Модель Claude для execute-фазы |
| 8 | Review agent models | — | `review_agents.*` | sonnet/haiku | Модель для каждого review-агента |
| 9 | Serena enabled | — | `serena_enabled` | true | Best effort Serena integration |
| 10 | Default branch | — | `default_branch` | auto-detect | База для git diff в review |
| 11 | Agent files dir | — | `agents_dir` | `.ralph/agents/` | Директория кастомных review agent `.md` файлов |
| 12 | Prompt file | — | `prompt_file` | `.ralph/prompts/execute.md` | Промпт для execute-фазы (override embedded default) |
| 13 | Claude command | — | `claude_command` | `claude` | Путь к Claude CLI |
| 14 | Paths | — | `paths.*` | defaults | sprint-tasks.md, LEARNINGS.md, CLAUDE.md |
| 15 | Always extract | `--always-extract` | `always_extract` | false | Extraction знаний после каждой итерации (не только failure/review) |
| 16 | Serena timeout | — | `serena_timeout` | 10 | Таймаут Serena incremental index (секунды). Full index при старте: 60s |

#### Agent files fallback chain

`.ralph/agents/` (project) > `~/.config/ralph/agents/` (global) > embedded defaults

### Output

| Канал | Формат | Назначение |
|-------|--------|------------|
| Terminal (stdout) | Цветной текст с progress indicators | Статус, human gates |
| `sprint-tasks.md` | Markdown | Задачи, статусы, AC |
| `LEARNINGS.md` | Markdown | Накопленные знания |
| `CLAUDE.md` | Markdown | Операционный контекст |
| `.ralph/logs/` | Text log | Полная история run для post-mortem |
| Git commits | Conventional commits | Результат задач/fixes |

### Exit Codes

| Code | Значение | Когда |
|------|----------|-------|
| 0 | Успех | Все задачи `[x]` |
| 1 | Частичный успех | Часть задач сделана, остановился по лимитам (gates off) |
| 2 | User quit | Пользователь выбрал quit на любом gate (обычном или emergency) |
| 3 | User interrupted | Ctrl+C (graceful shutdown) |
| 4 | Fatal error | Инфраструктурный сбой (нет git/claude, config, crash) |

### Dependencies

- `git`, `claude` CLI
- Ralph распространяется как single Go binary (zero runtime dependencies)

### Platform

- Linux, macOS — нативные бинарники
- Windows через WSL
- Распространение: `go install github.com/...` + GitHub Releases (goreleaser)
