# Оптимизация начального контекста Claude Code CLI

**Дата:** 2026-03-04
**Проект:** bmad-ralph
**Источников:** ~50 уникальных (GitHub issues, официальная документация, community research)

---

## 1. Проблема

Каждый вызов `claude -p` загружает **30-50K токенов overhead** до выполнения какой-либо работы. В автономном цикле (ralph loop), где N задач обрабатываются последовательно, этот overhead умножается N раз.

**Структура overhead (примерный breakdown для проекта с MCP):**

| Компонент | Токены | Доля |
|---|---|---|
| System prompt + built-in tools | 15-20K | 30-40% |
| Hosted MCP tools (Figma, Mermaid, Jam) | 10-16K | 20-30% |
| CLAUDE.md + `.claude/rules/` + MEMORY.md | 5-10K | 10-20% |
| Skills listing | 5-7K | 10-15% |
| Custom agents + git context + environment | 3-8K | 5-15% |

**Неустранимый минимум** без `--tools`: ~37-38K токенов (system prompt + все built-in tools + git context). С `--tools` (ограничение набора инструментов) + отключение MCP: ~23-24K. Ниже ~8K (system prompt + environment + CLAUDE.md) невозможно опуститься без `--system-prompt`.

**Фактор prompt caching:** система кэширует повторяющийся prefix (system prompt + tool definitions). Cached reads стоят 10% от обычных input tokens ($0.50/M vs $5/M). Но в ralph loop каждый свежий процесс теряет кэш, если prefix хотя бы минимально отличается.

---

## 2. Замеры на проекте bmad-ralph

Измерения выполнены 2026-03-04 через `claude -p "$(date +%N) Say HELLO" --verbose 2>&1`.

Формула: `total = cache_creation_input_tokens + cache_read_input_tokens + input_tokens`

| Конфигурация | Токены | Экономия |
|---|---|---|
| Baseline (bare `claude -p`) | 51,298 | -- |
| + CLI flags (`--tools`, `--plugin-dir`, `--setting-sources`, `--disable-slash-commands`) | 34,933 | **-32%** |
| + feature flag hack (`tengu_claudeai_mcp_connectors: false`) | 23,371 | **-54%** |
| + WebSearch/WebFetch добавлены обратно | 24,295 | **-53%** |

**Ключевой вывод:** комбинация CLI flags + отключение cloud MCP даёт **двукратное сокращение** начального контекста. Основной рычаг — MCP tool definitions (~16K токенов hosted MCP).

**Примечание:** Замеры выполнены с tools = `Bash,Edit,Read,Write,Grep,Glob,WebSearch,WebFetch` (execute set). Для review (+Task) и distillation (только Read,Write) токены будут отличаться на ±500-1500.

---

## 3. Работающие флаги и инструменты

### 3.1. `--tools` -- ограничение built-in tools

```bash
claude -p --tools "Bash,Edit,Read,Write,Grep,Glob" "prompt"
```

**Что делает:** Ограничивает набор built-in tools. Только перечисленные tools загружаются в контекст. Остальные полностью отсутствуют в system prompt.

**Экономия:** 2-5K токенов (зависит от числа убранных tools).

**Важно:** Это НЕ `--allowedTools`. `--allowedTools` контролирует permissions (какие tools работают без подтверждения), а `--tools` контролирует, какие tools вообще загружаются в контекст.

| Значение | Эффект |
|---|---|
| `--tools "Bash,Edit,Read"` | Только 3 tool в контексте |
| `--tools ""` | Все tools отключены (read-only mode) |
| `--tools "default"` | Все tools (по умолчанию) |

**Рекомендация по task type (bmad-ralph):**

| Тип задачи | Tools | Примечание |
|---|---|---|
| Execute (основной цикл) | `Bash,Edit,Read,Write,Grep,Glob` | Полный набор для редактирования |
| Review (code review) | `Task,Read,Grep,Glob,Bash` | **Task обязателен** — review.md запускает 5 sub-agents через Task tool |
| Knowledge distillation | `Read,Write` | MaxTurns=1, single-turn prompt/response, tools не используются |
| Resume extraction | `Read,Grep` | Парсинг предыдущей сессии |

**ВАЖНО:** `Task` (spawn sub-agent) и `TaskCreate/TaskUpdate/TaskList/TaskGet` (task tracking) — **разные** tools. Review нуждается в `Task`, но НЕ нуждается в Task* tracking tools.

### 3.2. `--plugin-dir /empty` -- блокировка plugins

```bash
mkdir -p /tmp/.empty-plugins
claude -p --plugin-dir /tmp/.empty-plugins "prompt"
```

**Что делает:** Указывает путь поиска plugins. Пустая директория = ни один plugin не загрузится.

**Экономия:** 5-15K токенов (plugin skill prompts могут быть очень объёмными).

**Ограничение:** Это Layer 3 из 4-layer isolation pattern. Без `--setting-sources project,local` (Layer 4) user-level settings могут переназначить plugin directory. Оба флага нужны вместе.

**Нет флага `--no-plugins`:** Issue [#20873](https://github.com/anthropics/claude-code/issues/20873) закрыт как NOT_PLANNED.

### 3.3. `--setting-sources project,local` -- ограничение settings

```bash
claude -p --setting-sources project,local "prompt"
```

**Что делает:** Контролирует, какие уровни settings загружаются. Значения: `user`, `project`, `local`. Опуская `user`, блокируем `~/.claude/settings.json` (hooks, enabledPlugins, MCP configs пользователя).

**Экономия:** Зависит от содержимого user settings. Предотвращает загрузку user-level plugins и MCP серверов.

**Caveat:** SDK issue [#186](https://github.com/anthropics/claude-agent-sdk-python/issues/186) -- старые версии SDK передают пустую строку в этот флаг, что ломает новые версии CLI.

### 3.4. `--disable-slash-commands` -- убираем skill definitions

```bash
claude -p --disable-slash-commands "prompt"
```

**Что делает:** Удаляет определения slash commands/skills из контекста.

**Экономия:** 1-3K токенов.

**Примечание:** Skills и так недоступны в `-p` mode, но без этого флага их определения всё равно загружаются в system prompt.

### 3.5. `--append-system-prompt` -- инъекция loop context

```bash
claude -p --append-system-prompt "Итерация 3/10. Осталось задач: 5. Предыдущая: scan tasks (OK)." "prompt"
```

**Что делает:** Добавляет текст к default system prompt. НЕ ломает prompt cache (appends ПОСЛЕ cached prefix).

**Экономия:** Нулевая (добавляет токены). Но это **правильный** способ передать loop state -- безопаснее, чем `--system-prompt`, который заменяет всё.

**Рекомендация:** 200-500 символов. Включать: номер итерации, число оставшихся задач, результат предыдущей (truncated до 200 chars).

### 3.6. `--max-turns` и `--max-budget-usd` -- safety limits

```bash
claude -p --max-turns 15 --max-budget-usd 2.00 "prompt"
```

**Что делает:** Ограничивает число agentic turns и бюджет в USD. Только для `-p` mode.

**Экономия:** Не снижает начальный контекст, но ограничивает суммарное потребление.

### 3.7. `--output-format json` -- structured output

```bash
claude -p --output-format json "prompt"
```

**Что делает:** Возвращает JSON с `session_id`, `result`, `cost_usd`, `duration_ms`, `num_turns`.

**Варианты:** `text`, `json`, `stream-json`. `stream-json` для real-time streaming.

### 3.8. `< /dev/null` -- предотвращение SIGTTIN hang

```bash
claude -p "prompt" < /dev/null
```

**Что делает:** Закрывает stdin. Предотвращает состояние **Tl** (stopped + multi-threaded), когда Claude Code пытается читать stdin во время MCP init.

**Почему нужно:** Claude Code использует Ink library для terminal UI. Даже в `-p` mode некоторые initialization paths (workspace trust check, permission prompt setup, MCP approval dialog) могут обратиться к `process.stdin`. Если stdin недоступен и процесс запущен как background -- ядро посылает SIGTTIN, процесс останавливается.

**Прецедент:** Vite 4+ имел точно такой же баг с SIGTTIN в background processes.

**Go-специфика (bmad-ralph):** В Go `exec.CommandContext` по умолчанию устанавливает `cmd.Stdin = nil`, что эквивалентно `/dev/null`. Ralph уже безопасен от SIGTTIN без дополнительных действий. Исключение: для промптов > 30K символов Ralph использует `cmd.Stdin = strings.NewReader(prompt)` (session.go:70-72) — stdin закрывается после чтения промпта, что тоже безопасно. Паттерн `< /dev/null` актуален только для bash-скриптов.

### 3.9. `ENABLE_CLAUDEAI_MCP_SERVERS=false` -- отключение cloud MCP

```bash
ENABLE_CLAUDEAI_MCP_SERVERS=false claude -p "prompt"
```

**Что делает:** Документированная env var для отключения hosted MCP connectors (Figma, Jam, Mermaid).

**Статус: ПРОТИВОРЕЧИВЫЕ РЕЗУЛЬТАТЫ.**
- Официально документирована в [Claude Code MCP docs](https://code.claude.com/docs/en/mcp)
- В тестах на mentorlearnplatform (2026-03-03): **не дала эффекта** (61,350 -> 61,350, токены переместились из cache_creation в cache_read)
- В тестах на bmad-ralph (2026-03-04): **дала эффект** в комбинации с feature flag hack

**Рекомендация:** Использовать, но не полагаться как единственный метод.

### 3.10. `MCP_TIMEOUT=N` -- timeout для MCP init

```bash
MCP_TIMEOUT=5000 claude -p "prompt"
```

**Что делает:** Устанавливает timeout в миллисекундах для MCP server startup.

**Зачем:** Предотвращает indefinite hang при инициализации MCP серверов (зафиксированы зависания до 16+ часов -- issue [#15945](https://github.com/anthropics/claude-code/issues/15945)).

### 3.11. Feature flag hack: `tengu_claudeai_mcp_connectors`

```json
// ~/.claude.json
{
  "cachedGrowthBookFeatures": {
    "tengu_claudeai_mcp_connectors": false
  }
}
```

**Что делает:** Отключает cloud MCP connector loading на уровне feature flags.

**Экономия:** ~16K токенов (все hosted MCP tools: Figma, Mermaid, Jam).

**Caveat:** **Перезаписывается** при обновлении Claude Code. Нужно устанавливать перед каждым вызовом или мониторить.

**В замерах:** Дал снижение с 34,933 до 23,371 токенов (-33% поверх CLI flags).

### 3.12. `CLAUDE_CODE_AUTOCOMPACT_PCT_OVERRIDE` -- порог компакции

```bash
CLAUDE_CODE_AUTOCOMPACT_PCT_OVERRIDE=60 claude -p "prompt"
```

**Что делает:** Управляет порогом автоматической компакции контекста. По умолчанию ~83.5% (33K буфер из 200K окна). Значение 50-65 вызывает более раннюю компакцию с лучшим сохранением quality.

**Исследование:** При 60-65% ещё возможны "clear, structured handovers with specific details". При 78% handover деградирует до "vague summaries missing critical state" (Stanford "Lost in the Middle", 15-47% performance drop).

---

## 4. Сломанные флаги (баги)

### 4.1. `--strict-mcp-config` + `--mcp-config` -- зависание

```bash
# ЭТО ЗАВИСАЕТ:
claude -p --strict-mcp-config --mcp-config '{}' "prompt"
```

**Issue:** [#18791](https://github.com/anthropics/claude-code/issues/18791) -- `--mcp-config` вызывает freeze с версии 2.1.7.

**Статус:** OPEN (reopened после первого закрытия).

**Детали:** Freeze происходит даже если JSON файл не существует. Поскольку `--strict-mcp-config` требует `--mcp-config` для работы, оба флага заблокированы этим багом.

**Это был бы главный инструмент:** `--strict-mcp-config --mcp-config '{}'` должен полностью отключать все MCP серверы. Экономия 10-51K токенов. Но не работает.

### 4.2. `--disallowedTools` с MCP tools -- молча игнорируется

```bash
# ЭТО НЕ РАБОТАЕТ:
claude -p --disallowedTools "mcp__claude_ai_Figma__get_screenshot" "prompt"
```

**Issue:** [#12863](https://github.com/anthropics/claude-code/issues/12863) -- закрыт как NOT_PLANNED (locked).

**Детали:** `--disallowedTools` работает корректно для built-in tools (Bash, Edit, Read), но **молча игнорируется** для всех MCP tools (`mcp__*`). Все форматы (один, через запятую, через пробел) -- не работают.

**Дубликаты:** #20617, #13077, #17567.

### 4.3. `--mcp-config` -- freeze regression

**Issue:** [#18791](https://github.com/anthropics/claude-code/issues/18791) -- OPEN.

**Affected versions:** 2.1.7, 2.1.11, 2.1.12.

**Платформы:** Linux, macOS.

Даже передача валидного JSON вызывает indefinite hang.

### 4.4. `ENABLE_CLAUDEAI_MCP_SERVERS=false` -- inconsistent behavior

**Статус:** Документирована, но при тестировании на mentorlearnplatform не дала эффекта. Total context не изменился (токены переместились между категориями кэширования, но суммарно остались те же).

### 4.5. Tl process state -- объяснение

Процесс Claude Code уходит в состояние **Tl** (по `ps aux`):
- **T** = Stopped by signal
- **l** = Multi-threaded (V8 threads -- нормально для Node.js)

**Причина:** SIGTTIN -- сигнал ядра, когда background process пытается читать stdin. Claude Code через Ink library может обращаться к `process.stdin` во время:
- Workspace trust check
- Permission prompt setup
- Ink raw mode initialization
- MCP server approval dialog

**Усугубляющий фактор для WSL2:** Задокументированы проблемы с signal propagation (microsoft/WSL#3766, microsoft/WSL#4914).

### 4.6. Полная карта багов

| Issue | Описание | Статус | Влияние |
|---|---|---|---|
| [#18791](https://github.com/anthropics/claude-code/issues/18791) | `--mcp-config` freeze | **OPEN** (reopened) | Блокирует `--strict-mcp-config` |
| [#12863](https://github.com/anthropics/claude-code/issues/12863) | `--disallowedTools` ignores MCP | Closed NOT_PLANNED | Флаг бесполезен для MCP |
| [#24481](https://github.com/anthropics/claude-code/issues/24481) | CLI hangs on simple queries | **OPEN** | MCP init blocking |
| [#25412](https://github.com/anthropics/claude-code/issues/25412) | Hangs loading MCP from api.anthropic.com | Closed (dupe #24481) | No timeout on fetch |
| [#15945](https://github.com/anthropics/claude-code/issues/15945) | MCP causes 16+ hour hang | **OPEN** | No per-server timeout |
| [#20412](https://github.com/anthropics/claude-code/issues/20412) | Cloud MCP auto-injected, OOM | **OPEN** (oncall) | Принудительная загрузка cloud MCP |
| [#14490](https://github.com/anthropics/claude-code/issues/14490) | `--strict-mcp-config` incomplete | Closed NOT_PLANNED | Неполная изоляция |
| [#9026](https://github.com/anthropics/claude-code/issues/9026) | CLI hangs without TTY | Closed NOT_PLANNED | TTY requirement bug |
| [#11898](https://github.com/anthropics/claude-code/issues/11898) | CLI suspends (setRawMode) | **OPEN** | Ink raw mode failure |
| [#20873](https://github.com/anthropics/claude-code/issues/20873) | `--no-mcp` feature request | Closed NOT_PLANNED | Нет простого disable |
| [#9996](https://github.com/anthropics/claude-code/issues/9996) | enabledPlugins: false не работает | **OPEN** | Disabled plugins загружают tool defs |
| [#27662](https://github.com/anthropics/claude-code/issues/27662) | `--no-hooks` feature request | **OPEN** | Нет workaround |

---

## 5. Что убрали и что осталось

### Замеры bmad-ralph: baseline (51,298) vs optimized (24,295)

| Категория | Baseline | Optimized | Статус |
|---|---|---|---|
| System prompt (base) | ~269 | ~269 | **Остаётся** (unavoidable) |
| Default prompt modules | ~8,000 | ~8,000 | **Остаётся** (с `--append-system-prompt`) |
| Bash, Edit, Read, Write, Grep, Glob | ~3,000 | ~3,000 | **Остаётся** (нужны для работы) |
| WebSearch, WebFetch | ~900 | ~900 | **Добавлены обратно** (нужны) |
| NotebookEdit, EnterWorktree | ~400 | 0 | **Убрано** |
| Task (spawn sub-agent) | ~500 | ~500 | **Нужен для review** (5 sub-agents) |
| TeamCreate, TeamDelete, SendMessage | ~600 | 0 | **Убрано** |
| TaskCreate, TaskUpdate, TaskList, TaskGet | ~800 | 0 | **Убрано** (task tracking не нужен) |
| AskUserQuestion, EnterPlanMode | ~400 | 0 | **Убрано** (headless mode) |
| Hosted MCP: Figma (~31 tool) | ~8,000 | 0 | **Убрано** (feature flag) |
| Hosted MCP: Mermaid Chart (~4 tool) | ~3,000 | 0 | **Убрано** (feature flag) |
| Hosted MCP: Jam (~12 tool) | ~5,000 | 0 | **Убрано** (feature flag) |
| Plugin skills | ~5,000 | 0 | **Убрано** (`--plugin-dir`) |
| Slash command definitions | ~2,000 | 0 | **Убрано** (`--disable-slash-commands`) |
| User settings (hooks, etc.) | ~1,000 | 0 | **Убрано** (`--setting-sources`) |
| CLAUDE.md + rules | ~5,000 | ~5,000 | **Остаётся** (нужно для проекта) |

**Что теряем при оптимизации:**
- MCP tools (Figma, Mermaid, Jam) — не нужны в ralph loop
- Plugin skills — не нужны в headless mode
- Task tracking tools (TaskCreate/Update/List/Get) — loop orchestrator управляет задачами сам
- Team tools (TeamCreate/Delete, SendMessage) — не используются
- Interactive tools (AskUserQuestion, EnterPlanMode) — headless mode
- NotebookEdit, EnterWorktree — не используются в Go проекте

**Что сохраняем:**
- **Task** (sub-agent spawn) — **обязателен для review** workflow (5 sub-agents)
- Bash, Edit, Read, Write, Grep, Glob — для execute
- WebSearch, WebFetch — для поиска в интернете (опционально)

---

## 6. Рекомендуемая конфигурация для bmad-ralph

### Tool sets по типу сессии

Ralph запускает 4 типа Claude-сессий. Каждому нужен свой набор tools:

```bash
# Execute (основной цикл разработки)
TOOLS_EXECUTE="Bash,Edit,Read,Write,Grep,Glob,WebSearch,WebFetch"

# Review (code review с 5 sub-agents)
TOOLS_REVIEW="Task,Read,Grep,Glob,Bash"

# Knowledge distillation (single-turn, MaxTurns=1)
TOOLS_DISTILL="Read,Write"

# Resume extraction (парсинг предыдущей сессии)
TOOLS_RESUME="Read,Grep"
```

### Полная команда запуска (execute пример)

```bash
# Feature flag: отключить cloud MCP (может сброситься при обновлении)
# Нужно устанавливать перед каждым вызовом если не уверены
jq '.cachedGrowthBookFeatures.tengu_claudeai_mcp_connectors = false' \
  ~/.claude.json > /tmp/claude.json.tmp && mv /tmp/claude.json.tmp ~/.claude.json

# Запуск claude -p с полной изоляцией
ENABLE_CLAUDEAI_MCP_SERVERS=false \
MCP_TIMEOUT=10000 \
  claude -p \
  --tools "${TOOLS_EXECUTE}" \
  --plugin-dir /tmp/.empty-plugins \
  --setting-sources project,local \
  --disable-slash-commands \
  --append-system-prompt "Итерация ${ITER}/${MAX_ITER}. Задач осталось: ${REMAINING}." \
  --max-turns 15 \
  --max-budget-usd 3.00 \
  --output-format json \
  --dangerously-skip-permissions \
  "Task prompt here" < /dev/null
```

### Go-имплементация (session.go)

В Go коде Ralph env vars передаются через `cmd.Env`:

```go
cmd.Env = append(os.Environ(),
    "ENABLE_CLAUDEAI_MCP_SERVERS=false",
    "MCP_TIMEOUT=10000",
)
```

`< /dev/null` не нужен — Go `exec.CommandContext` по умолчанию устанавливает `cmd.Stdin = nil` (эквивалент /dev/null). Для длинных промптов stdin используется для доставки промпта (session.go:70-72).

### Подготовка environment

```bash
# Создать пустую директорию для plugins (один раз)
mkdir -p /tmp/.empty-plugins

# Убедиться, что feature flag установлен
python3 -c "
import json, os
p = os.path.expanduser('~/.claude.json')
with open(p) as f: d = json.load(f)
d.setdefault('cachedGrowthBookFeatures', {})['tengu_claudeai_mcp_connectors'] = False
with open(p, 'w') as f: json.dump(d, f, indent=2)
"
```

### Что НЕ использовать

| Флаг | Причина |
|---|---|
| `--strict-mcp-config` | Freeze bug [#18791](https://github.com/anthropics/claude-code/issues/18791) |
| `--mcp-config` | Freeze regression [#18791](https://github.com/anthropics/claude-code/issues/18791) |
| `--disallowedTools "mcp__*"` | Молча игнорируется [#12863](https://github.com/anthropics/claude-code/issues/12863) |

### Примечание о Serena MCP

Если целевой проект использует Serena MCP (`.mcp.json` или `.claude/settings.json`), local MCP серверы **продолжат загружаться** -- мы не используем `--strict-mcp-config`. Блокируются только cloud-hosted connectors (Figma, Jam, Mermaid) через `ENABLE_CLAUDEAI_MCP_SERVERS=false` и feature flag.

---

## 7. Паттерны сообщества

### 7.1. Fresh context per task (6/8 implementations)

Доминирующий паттерн. Каждая задача выполняется в новом процессе с чистым контекстом. Предотвращает degradation от длинных сессий.

```
while tasks_remain:
    task = get_next_task()
    result = claude -p "task prompt" < /dev/null
    update_state(result)
    commit_if_needed()
```

**Кто использует:** claude-loop, ralphex, continuous-claude, ralph-loop-skills, ralph-claude-code, bmad-ralph.

**Философия:** "Claude doesn't need to remember the whole project -- it needs to focus on one task at a time."

### 7.2. External markdown state files

Состояние loop хранится в markdown файлах на диске, а не в conversation history:

- `SHARED_TASK_NOTES.md` -- "relay race" pattern (continuous-claude)
- `active-plan.md` с checkboxes `[x]` / `[ ]` (ralph-loop-skills)
- `fix_plan.md` + `specs/*.md` (ralph-claude-code)
- `progress.txt` + task state JSON (self-improving agents)

**Ключевой insight:** "Think of it as a relay race where you're passing the baton."

### 7.3. `--append-system-prompt` для loop context (200-500 chars)

```bash
build_loop_context() {
    echo "Loop: ${LOOP_NUM}/${MAX_LOOPS}"
    echo "Remaining tasks: ${TASK_COUNT}"
    echo "Previous: $(echo "$PREV_RESULT" | head -c 200)"
}

claude -p --append-system-prompt "$(build_loop_context)" "prompt"
```

Не ломает prompt cache (appends ПОСЛЕ cached prefix). Достаточно для orientation без перегрузки контекста.

### 7.4. Session resume vs fresh -- tradeoffs

| Аспект | Fresh context | `--resume` |
|---|---|---|
| Context quality | Максимальная | Деградирует после 60-65% usage |
| Cold start | Полный (30-50K) | Нулевой |
| Cache | Miss на первом turn | Hit (prefix reused) |
| Complexity | Минимальная | Нужен session_id management |
| Adoption | 6/8 tools | 4/8 tools |

**Оптимальная стратегия:** Fresh sessions per task. `--resume` только для продолжения внутри одной задачи (не между задачами).

### 7.5. 4-layer subprocess isolation pattern

Из [DEV.to research](https://dev.to/jungjaehoon/why-claude-code-subagents-waste-50k-tokens-per-turn-and-how-to-fix-it-41ma):

| Layer | Механизм | Что блокирует |
|---|---|---|
| 1 | Scoped working directory | Auto-load `~/CLAUDE.md` |
| 2 | Git boundary (`.git/HEAD` в cwd) | Upward CLAUDE.md traversal |
| 3 | `--plugin-dir /empty/dir` | Plugin skill injection |
| 4 | `--setting-sources project,local` | User-level settings re-enabling |

**Результат:** Turn 1: 50K -> 5K токенов. 5 turns cumulative: 250K -> 25K (10x).

### 7.6. Context rotation at 60-65%

Из [исследования Vincent Van Deth](https://vincentvandeth.nl/blog/context-rot-claude-code-automatic-rotation):

- Auto-compaction срабатывает при ~83.5% (слишком поздно)
- Оптимальная ротация: **60-65%** context usage
- При 60%: "clear, structured handovers with specific details"
- При 78%: "vague summaries missing critical state information"
- Stanford "Lost in the Middle": 15-47% performance drop с ростом контекста

**Для ralph loop:** `CLAUDE_CODE_AUTOCOMPACT_PCT_OVERRIDE=60` или ограничивать `--max-turns`.

### 7.7. Cost data points

| Метрика | Значение |
|---|---|
| Per-iteration cost (continuous-claude) | ~$0.042 |
| Anthropic official average | $6/dev/day, $12/day P90 |
| Monthly estimate (Sonnet 4.6) | $100-200/dev/month |
| Prompt caching savings | 90% on cached tokens |
| Without caching (100 turns Opus) | $50-100 |
| With caching (same session) | $10-19 |
| Agent teams token multiplier | ~7x vs standard |
| Multi-turn overhead | +30-50% per additional turn |
| Usable context | ~176K of 200K (tool output takes rest) |

### 7.8. Loop implementations overview

| Инструмент | Язык | Подход | CLI flags |
|---|---|---|---|
| [claude-loop](https://github.com/li0nel/claude-loop) | Bash | Fresh context, cost tracking | `--output-format stream-json` |
| [ralphex](https://github.com/umputun/ralphex) | Go | 4-phase pipeline, 5 review agents | `--dangerously-skip-permissions` |
| [ralph-claude-code](https://github.com/frankbria/ralph-claude-code) | Bash | Session resume, exit detection | `--resume`, `--append-system-prompt` |
| [continuous-claude](https://github.com/AnandChowdhary/continuous-claude) | Bash/TS | CI integration, cost limits | `--max-runs`, `--max-cost` |
| [claude-code-toolkit](https://github.com/intellegix/claude-code-toolkit) | JS/TS | NDJSON streaming, metrics | `--resume`, `--output-format stream-json` |
| [claude-autopilot](https://pkg.go.dev/github.com/hseinmoussa/claude-autopilot) | Go | YAML task queue, rate limits | `--resume`, `--model` |
| [ralph-loop-skills](https://github.com/tradesdontlie/ralph-loop-skills) | Bash | Checkbox state machine | `--dangerously-skip-permissions` |
| [motlin approach](https://motlin.com/blog/claude-code-running-for-hours) | Claude Code | Hierarchical agents | Sub-agent delegation |

---

## 8. Что ещё можно попробовать

### 8.1. `--system-prompt` -- замена всего default prompt

```bash
claude -p --system-prompt "You are a focused Go code assistant. Follow instructions exactly." "prompt"
```

**Экономия:** 5-10K токенов (весь default prompt заменяется).

**Риск:** HIGH. Теряются все default instructions для tool orchestration. Claude может хуже работать с multi-file edits, error recovery, git operations.

**Когда использовать:** Только для простых single-tool tasks ("read file X and summarize").

### 8.2. `--input-format stream-json` -- persistent process

```bash
claude -p \
  --input-format stream-json \
  --output-format stream-json \
  --session-id "$(uuidgen)" \
  --tools "Bash,Edit,Read,Write,Grep,Glob"
```

System prompt загружается **один раз**. Messages идут через stdin/stdout. Для N задач экономия = (N-1) * cold_start_overhead.

**Сложность:** Высокая. Требуется:
- JSON message framing через stdin
- Process lifecycle management
- Context rotation при 60-65% usage
- Error recovery при crash

### 8.3. Agent SDK вместо CLI

[Claude Agent SDK](https://github.com/anthropics/claude-agent-sdk-python) (Python):

```python
from claude_agent_sdk import AgentSession

session = AgentSession(
    model="claude-sonnet-4-6",
    tools=["Bash", "Read", "Edit"],
    system_prompt="Minimal instructions"
)
result = session.send("task prompt")
```

**Преимущества:** Minimal system prompt по умолчанию. Полный контроль над tools и context. Нет CLI overhead.

**Недостатки:** Теряется вся Claude Code экосистема (hooks, settings, CLAUDE.md auto-load, skills).

### 8.4. Selective CLAUDE.md loading

Текущее поведение: CLAUDE.md загружается целиком. `.claude/rules/*.md` загружаются по glob scope hints.

**Возможная оптимизация:**
- Держать CLAUDE.md минимальным (< 80 строк, текущий bmad-ralph: ~65)
- Детальные правила в `.claude/rules/*.md` с scope hints
- Trigger tables вместо verbose documentation (54% reduction, John Lindquist)

### 8.5. `.claudeignore`

```
# .claudeignore
node_modules/
dist/
build/
coverage/
*.min.js
vendor/
```

**Экономия:** 25-100K токенов на file-reading operations (не на начальный контекст).

**Для bmad-ralph:** Go проект без node_modules, но `cover.html` и `cover_de5Xa.html` можно добавить.

---

## 9. Источники

### Официальная документация Anthropic

1. [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
2. [Headless/Programmatic Mode](https://code.claude.com/docs/en/headless)
3. [Settings Documentation](https://code.claude.com/docs/en/settings)
4. [Cost Management](https://code.claude.com/docs/en/costs)
5. [MCP Documentation](https://code.claude.com/docs/en/mcp)
6. [Plugins Reference](https://code.claude.com/docs/en/plugins-reference)
7. [Permissions](https://code.claude.com/docs/en/permissions)
8. [Prompt Caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)

### Community research и инструменты

9. [DEV.to: Why Each Subprocess Burns 50K Tokens](https://dev.to/jungjaehoon/why-claude-code-subagents-waste-50k-tokens-per-turn-and-how-to-fix-it-41ma) -- 4-layer isolation pattern
10. [Gist: 54% Context Reduction](https://gist.github.com/johnlindquist/849b813e76039a908d962b2f0923dc9a) -- trigger tables + lazy loading
11. [SDK vs CLI System Prompts](https://github.com/shanraisshan/claude-code-best-practice/blob/main/reports/claude-agent-sdk-vs-cli-system-prompts.md) -- token overhead comparison
12. [MCP Tool Search 95% Savings](https://claudefa.st/blog/tools/mcp-extensions/mcp-tool-search) -- ENABLE_TOOL_SEARCH analysis
13. [Context Buffer 33K Problem](https://claudefa.st/blog/guide/mechanics/context-buffer-management) -- compaction threshold internals
14. [Context Rot & Rotation](https://vincentvandeth.nl/blog/context-rot-claude-code-automatic-rotation) -- 60-65% optimal rotation
15. [System Prompt Catalog](https://github.com/Piebald-AI/claude-code-system-prompts) -- 110+ prompt components
16. [Self-Improving Agents](https://addyosmani.com/blog/self-improving-agents/) -- memory persistence patterns
17. [Prompt Caching in Claude Code](https://www.claudecodecamp.com/p/how-prompt-caching-actually-works-in-claude-code) -- cache mechanics
18. [Claude Code Behind Scenes](https://blog.promptlayer.com/claude-code-behind-the-scenes-of-the-master-agent-loop/) -- architecture analysis
19. [Stream-JSON Chaining](https://github.com/ruvnet/claude-flow/wiki/Stream-Chaining) -- persistent process pattern

### Loop implementations

20. [claude-loop](https://github.com/li0nel/claude-loop) -- Bash, fresh context, cost tracking
21. [ralphex](https://github.com/umputun/ralphex) -- Go, 4-phase pipeline
22. [ralph-claude-code](https://github.com/frankbria/ralph-claude-code) -- Bash, session resume
23. [continuous-claude](https://github.com/AnandChowdhary/continuous-claude) -- Bash/TS, CI integration
24. [claude-code-toolkit](https://github.com/intellegix/claude-code-toolkit) -- JS/TS, NDJSON metrics
25. [claude-autopilot](https://pkg.go.dev/github.com/hseinmoussa/claude-autopilot) -- Go, YAML task queue
26. [ralph-loop-skills](https://github.com/tradesdontlie/ralph-loop-skills) -- Bash, checkbox state
27. [motlin: Claude Running for Hours](https://motlin.com/blog/claude-code-running-for-hours) -- hierarchical agents

### GitHub issues (claude-code)

28. [#18791](https://github.com/anthropics/claude-code/issues/18791) -- `--mcp-config` freeze (OPEN)
29. [#12863](https://github.com/anthropics/claude-code/issues/12863) -- `--disallowedTools` ignores MCP (NOT_PLANNED)
30. [#24481](https://github.com/anthropics/claude-code/issues/24481) -- CLI hangs on queries (OPEN)
31. [#15945](https://github.com/anthropics/claude-code/issues/15945) -- MCP 16+ hour hang (OPEN)
32. [#20412](https://github.com/anthropics/claude-code/issues/20412) -- Cloud MCP auto-injected (OPEN)
33. [#14490](https://github.com/anthropics/claude-code/issues/14490) -- `--strict-mcp-config` incomplete (NOT_PLANNED)
34. [#20873](https://github.com/anthropics/claude-code/issues/20873) -- `--no-mcp` request (NOT_PLANNED)
35. [#9026](https://github.com/anthropics/claude-code/issues/9026) -- CLI hangs without TTY (NOT_PLANNED)
36. [#11898](https://github.com/anthropics/claude-code/issues/11898) -- CLI suspends setRawMode (OPEN)
37. [#27662](https://github.com/anthropics/claude-code/issues/27662) -- `--no-hooks` request (OPEN)
38. [#6759](https://github.com/anthropics/claude-code/issues/6759) -- Disable MCP tools context
39. [#2692](https://github.com/anthropics/claude-code/issues/2692) -- `--system-prompt` in interactive
40. [#27645](https://github.com/anthropics/claude-code/issues/27645) -- Sub-agent token waste
41. [#13761](https://github.com/anthropics/claude-code/issues/13761) -- Task tool 77K tokens
42. [#186 (SDK)](https://github.com/anthropics/claude-agent-sdk-python/issues/186) -- `--setting-sources` empty string bug

### Дополнительные источники

43. [12 Token-Saving Techniques](https://aslamdoctor.com/12-proven-techniques-to-save-tokens-in-claude-code/)
44. [Token Management 50-70%](https://dev.to/richardporter/claude-code-token-management-8-strategies-to-save-50-70-on-pro-plan-3hob)
45. [MCP Context Isolation](https://paddo.dev/blog/claude-code-mcp-context-isolation/)
46. [Sub-agent Token Burn](https://dev.to/onlineeric/claude-code-sub-agents-burn-out-your-tokens-4cd8)
47. [Vite SIGTTIN Bug](https://github.com/Bourg/vite-background-process-sigttin) -- прецедент для stdin hang
48. [WSL #3766](https://github.com/microsoft/WSL/issues/3766) -- SIGINT not propagated in WSL
49. [WSL #4914](https://github.com/microsoft/WSL/issues/4914) -- Node.js unkillable in WSL
