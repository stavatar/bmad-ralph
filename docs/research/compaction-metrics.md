# Исследование: метрики компакта и контекстного окна для Ralph

Дата: 2026-03-06

## 1. Findings

### 1.1. Как получить ТОЧНОЕ количество компактов на итерацию

#### stream-json формат (Вариант A — рекомендуемый)

Claude Code поддерживает `--output-format stream-json`, который выдаёт NDJSON (newline-delimited JSON). При этом формате существуют следующие типы сообщений:

| Тип сообщения | Описание |
|:---|:---|
| `SystemMessage` (subtype `init`) | Инициализация сессии: session_id, model, tools, версия |
| `AssistantMessage` | Полные ответы Claude (текст и/или tool use) |
| `UserMessage` (tool results) | Результаты вызовов инструментов |
| `StreamEvent` | Потоковые события (требует `--include-partial-messages`) |
| **`CompactBoundaryMessage`** | **Маркер компакта — появляется когда conversation history была компактирована** |
| `ResultMessage` | Финальный результат с usage/cost/num_turns |

**Ключевое открытие**: тип `CompactBoundaryMessage` — прямой маркер события компакта. Каждое появление этого сообщения в NDJSON-потоке означает одну операцию компакта. Подсчёт этих сообщений даёт ТОЧНОЕ количество компактов.

Источник: [Agent SDK Streaming Output](https://platform.claude.com/docs/en/agent-sdk/streaming-output) — "Without partial messages enabled, you receive all message types except StreamEvent. Common types include SystemMessage, AssistantMessage, ResultMessage, and **CompactBoundaryMessage** (indicates when conversation history was compacted)."

#### PreCompact hook (Вариант B — дополнительный)

Claude Code поддерживает hook `PreCompact` ([Hooks reference](https://code.claude.com/docs/en/hooks)), который срабатывает перед компактом:

```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/project",
  "hook_event_name": "PreCompact",
  "trigger": "auto",
  "custom_instructions": ""
}
```

Matcher: `auto` (автоматический) или `manual` (пользователь вызвал `/compact`).

**Ограничения PreCompact**: нет decision control (нельзя заблокировать компакт), выход hook'а попадает в контекст который будет суммаризирован. PostCompact hook НЕ существует ([issue #14258](https://github.com/anthropics/claude-code/issues/14258)).

#### --verbose флаг

Флаг `--verbose` включает подробный turn-by-turn вывод, но НЕ добавляет структурированные данные о компакте. Полезен только для отладки.

#### --debug флаг

Флаг `--debug` поддерживает категории фильтрации (например `"api,hooks"`), но документация не описывает специфичных категорий для компакта.

#### CLAUDE_AUTOCOMPACT_PCT_OVERRIDE

Env var `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` позволяет настраивать порог автокомпакта (1-100%). По умолчанию ~83.5% (200K * 0.835 ≈ 167K токенов). Ранее было ~77-78%.

### 1.2. Как получить средний % заполнения контекстного окна

#### Данные в stream-json ResultMessage

`ResultMessage` содержит:
- `usage.input_tokens` — не-кешированные входные токены
- `usage.cache_read_input_tokens` — кешированные токены
- `usage.cache_creation_input_tokens` — токены записанные в кеш
- `usage.output_tokens` — выходные токены
- `num_turns` — количество turns
- `total_cost_usd` — стоимость

Но это **кумулятивные** данные за всю сессию, а не per-turn.

#### Данные в stream-json AssistantMessage (per-turn)

В stream-json каждый `AssistantMessage` содержит `usage` объект с per-turn token counts. Это позволяет отслеживать заполнение контекста на каждом turn'е.

#### StatusLine данные (не доступны в -p режиме)

StatusLine hook получает объект `context_window`:

```json
{
  "context_window": {
    "total_input_tokens": 15234,
    "total_output_tokens": 4521,
    "context_window_size": 200000,
    "used_percentage": 8,
    "remaining_percentage": 92,
    "current_usage": {
      "input_tokens": 8500,
      "output_tokens": 1200,
      "cache_creation_input_tokens": 5000,
      "cache_read_input_tokens": 2000
    }
  }
}
```

`used_percentage` вычисляется из input tokens: `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`. Но StatusLine работает только в интерактивном режиме, не в `-p` (print mode).

#### Формула расчёта context fill из per-turn usage

Для каждого API call (turn):
```
context_fill = (input_tokens + cache_creation_input_tokens + cache_read_input_tokens) / context_window_size * 100
```

Где `context_window_size` = 200000 (из `ResultMessage.modelUsage`).

**Важно**: `cache_read_input_tokens` и `cache_creation_input_tokens` — это токены которые занимают место в контекстном окне, хоть и дешевле по стоимости. Все три типа входных токенов суммируются для определения заполнения контекста ([codelynx.dev](https://codelynx.dev/posts/calculate-claude-code-context)).

### 1.3. Архитектурные варианты интеграции

#### Вариант A: stream-json + парсинг CompactBoundaryMessage (РЕКОМЕНДУЕМЫЙ)

Переключить Ralph с `--output-format json` на `--output-format stream-json`:

**Плюсы**:
- ТОЧНОЕ количество компактов (CompactBoundaryMessage)
- Per-turn token usage из каждого AssistantMessage
- Возможность вычислить max context fill за сессию
- Real-time мониторинг (не обязательно использовать)

**Минусы**:
- Значительный рефакторинг `session.Execute()` и `session.ParseResult()` — нужен потоковый парсинг NDJSON вместо буферизации stdout
- NDJSON парсинг сложнее, чем одиночный JSON объект
- Нужно аккумулировать `AssistantMessage` events для сборки итогового результата
- ResultMessage по-прежнему содержит те же данные что и JSON format

#### Вариант B: PreCompact hook + счётчик в файле

Настроить PreCompact hook (с matcher `auto`) который инкрементирует счётчик в файле:

```bash
#!/bin/bash
# .claude/hooks/count-compact.sh
COUNTER_FILE="$HOME/.ralph/compact-count-$(date +%s).txt"
echo 1 >> "$COUNTER_FILE"
```

Ralph читает файл после завершения сессии.

**Плюсы**:
- Минимальные изменения в Ralph (только чтение файла после сессии)
- Работает с текущим `--output-format json`
- Hook описывается в проектных settings.json

**Минусы**:
- Нет per-turn context fill (только факт компакта)
- Сложность управления lifecycle файла-счётчика
- Зависимость от внешнего hook (не self-contained)
- Нет PostCompact hook — не можем узнать состояние ПОСЛЕ компакта

#### Вариант C: Косвенный расчёт из имеющихся метрик

Использовать только данные из текущего JSON result:

```
estimated_compactions = max(0, (cache_read_tokens / num_turns) / context_window - 1)
```

**Плюсы**:
- Ноль изменений в Ralph
- Мгновенная реализация

**Минусы**:
- Грубая оценка, не точное количество
- `cache_read_input_tokens` кумулятивный за ВСЮ сессию — не даёт per-turn data
- При компакте token counts сбрасываются, что ломает формулу

#### Вариант D: stream-json ТОЛЬКО для подсчёта CompactBoundaryMessage

Гибрид: парсить NDJSON поток ТОЛЬКО для подсчёта `CompactBoundaryMessage` и извлечения per-turn usage, а финальный результат брать из `ResultMessage` (который имеет ту же структуру что и текущий JSON output).

**Плюсы**:
- ТОЧНЫЙ подсчёт компактов
- Per-turn context fill
- Рефакторинг изолирован в `session.Execute()` — ParseResult работает с ResultMessage
- Не нужно менять формат выхода Runner'а

**Минусы**:
- Всё равно нужен потоковый парсинг NDJSON (но проще чем полный Вариант A)

### 1.4. Анализ текущих данных mentorlearnplatform

Данные из реального run (11 execute + 11 review сессий):

#### Execute сессии

| # | num_turns | cache_read | input | output | cost | cache_read/turn |
|---|-----------|------------|-------|--------|------|-----------------|
| 1 | 14 | 618,042 | 33,820 | 8,001 | $0.34 | 44,146 |
| 2 | 20 | 956,133 | 45,920 | 13,028 | $0.56 | 47,807 |
| 3 | 27 | 1,418,382 | 65,073 | 14,684 | $0.82 | 52,533 |
| 4 | 27 | 1,427,598 | 65,428 | 16,060 | $0.84 | 52,874 |
| 5 | 31 | 1,631,283 | 88,785 | 19,497 | $1.00 | 52,622 |
| 6 | 33 | 1,807,800 | 95,345 | 23,498 | $1.14 | 54,782 |
| 7 | 35 | 2,060,710 | 99,285 | 21,024 | $1.26 | 58,877 |
| 8 | 37 | 2,255,361 | 113,218 | 19,818 | $1.36 | 60,956 |
| 9 | 38 | 2,340,474 | 117,483 | 26,474 | $1.47 | 61,591 |
| 10 | 39 | 2,752,839 | 116,011 | 23,254 | $1.63 | 70,585 |
| 11 | 41 | 3,149,106 | 137,637 | 33,618 | $2.35 | 76,808 |

#### Анализ: происходит ли компакт?

**Критический показатель**: cache_read_input_tokens/num_turns — это СРЕДНИЙ размер кешированного контекста на один turn. Это ближайшая оценка среднего заполнения контекста.

Для сессий с 35+ turns:
- Средний cache_read/turn: 60K-77K токенов
- Но это ТОЛЬКО cached portion. Реальный контекст = cache_read + cache_creation + input (per turn)
- При context window = 200K и auto-compact threshold ≈ 167K токенов

**Оценка per-turn context fill (грубая)**:

Для execute #11 (41 turn, $2.35):
- Общий cache_read: 3,149,106
- Кумулятивные input: 137,637 (нарастающий итог ~3,355 per turn average)
- cache_read/turn: 76,808

Если предположить что на последних turns cache_read/turn растёт линейно, то на 40-м turn'е context fill ≈ 100K-120K токенов. Это **ниже** порога компакта (167K).

**Однако** cache_read_input_tokens — КУМУЛЯТИВНАЯ сумма за ВСЮ сессию. Если было N turns, то:
```
total_cache_read = sum(per_turn_cache_read[i] for i in 1..N)
```

Каждый turn "прочитывает" весь текущий контекст из кеша + новый input. Если контекст растёт от 20K до 120K за 40 turns, средний per-turn будет ~70K, и total ≈ 70K * 40 = 2.8M — что совпадает с данными.

**Вывод**: на основе имеющихся данных, execute сессии до 41 turn'а ВЕРОЯТНО не достигают порога компакта (167K). Но для сессий с cache_read_input_tokens > 3M (последние execute) контекст приближается к 80-100K на последних turns, что составляет 40-50% context window.

#### Review сессии

Review сессии показывают схожую картину:
- 14-31 turn, cache_read 618K-1.6M
- cache_read/turn: 44K-52K — значительно ниже порога

**Вывод по review**: компакт в review сессиях маловероятен.

#### Ключевая неопределённость

Без per-turn данных мы НЕ МОЖЕМ точно определить:
1. Максимальный context fill на каком-либо turn'е
2. Был ли хотя бы один компакт (cache_read сбрасывается при компакте)
3. Распределение context fill по turns (линейный рост vs скачки)

Только Вариант A или D даст точные данные.

## 2. Recommended Approach

### Рекомендация: Вариант D — stream-json для CompactBoundaryMessage + per-turn usage

**Обоснование**:

1. **Точность**: единственный способ получить ТОЧНОЕ количество компактов — `CompactBoundaryMessage`
2. **Per-turn data**: stream-json даёт per-turn `usage` в каждом `AssistantMessage`, позволяя вычислить max context fill
3. **Минимальный scope**: ResultMessage в stream-json имеет ту же структуру что и текущий JSON output, поэтому `ParseResult` можно адаптировать с минимальными изменениями
4. **Будущие возможности**: потоковый парсинг открывает путь к real-time мониторингу (kill сессии при превышении порога)

**Не рекомендуется Вариант B** (PreCompact hook) — потому что:
- Нет PostCompact hook, нет per-turn данных
- Внешняя зависимость (hook файл вне контроля Ralph)
- Сложность lifecycle управления

**Не рекомендуется Вариант C** (косвенный расчёт) — потому что:
- Неточен принципиально
- Не различает "контекст рос линейно до 80K" от "контекст дошёл до 167K и компактнулся до 40K"

## 3. Implementation Sketch

### Фаза 1: Новые структуры данных

#### session/result.go — новые поля

```go
// SessionResult — добавить поля:
type SessionResult struct {
    // ... existing fields ...
    CompactionCount int   // Количество CompactBoundaryMessage в сессии
    MaxContextFill  int   // Максимальный context fill (tokens) за сессию
    ContextFillPct  float64 // MaxContextFill / ContextWindowSize * 100
}
```

#### runner/metrics.go — новые поля

```go
// TaskMetrics — добавить:
type TaskMetrics struct {
    // ... existing fields ...
    TotalCompactions int     `json:"total_compactions"`  // Сумма компактов по всем сессиям задачи
    MaxContextPct    float64 `json:"max_context_pct"`    // Макс % заполнения контекста
    AvgContextPct    float64 `json:"avg_context_pct"`    // Средний % заполнения контекста
}

// RunMetrics — добавить:
type RunMetrics struct {
    // ... existing fields ...
    TotalCompactions int     `json:"total_compactions"`
    MaxContextPct    float64 `json:"max_context_pct"`
}
```

### Фаза 2: Потоковый парсинг NDJSON

#### session/session.go — изменения в Execute()

Текущая реализация буферизирует stdout целиком:
```go
var stdoutBuf, stderrBuf bytes.Buffer
cmd.Stdout = &stdoutBuf
cmd.Stderr = &stderrBuf
```

Новая реализация использует io.Pipe + goroutine для потокового парсинга:

```go
// Pipe stdout через NDJSON парсер
pr, pw := io.Pipe()
cmd.Stdout = pw

// StreamAccumulator собирает данные из NDJSON потока
acc := &StreamAccumulator{}
done := make(chan error, 1)
go func() {
    done <- acc.Process(pr) // Парсит каждую строку, считает CompactBoundaryMessage
}()

err := cmd.Run()
pw.Close()
<-done
```

#### session/stream.go — новый файл

```go
package session

// StreamAccumulator парсит NDJSON поток из Claude Code stream-json.
type StreamAccumulator struct {
    CompactionCount int
    MaxContextFill  int       // max tokens across all turns
    ContextWindow   int       // from init message (200000)
    LastResult      *jsonResultMessage
    perTurnUsage    []turnUsage
}

type turnUsage struct {
    InputTokens         int
    CacheReadTokens     int
    CacheCreationTokens int
    OutputTokens        int
}

// Process читает NDJSON поток и аккумулирует метрики.
func (sa *StreamAccumulator) Process(r io.Reader) error {
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
        line := scanner.Bytes()
        var msg struct {
            Type string `json:"type"`
        }
        if json.Unmarshal(line, &msg) != nil {
            continue
        }
        switch msg.Type {
        case "compact_boundary":
            sa.CompactionCount++
        case "assistant":
            sa.processAssistantMessage(line)
        case "result":
            sa.processResultMessage(line)
        case "system":
            sa.processSystemMessage(line)
        }
    }
    return scanner.Err()
}
```

### Фаза 3: Изменения в buildArgs

```go
// session/session.go — buildArgs():
// Заменить:
//   args = append(args, flagOutputFormat, outputFormatJSON)
// На:
//   args = append(args, flagOutputFormat, outputFormatStreamJSON)
```

### Фаза 4: Интеграция с MetricsCollector

```go
// runner/metrics.go — RecordSession():
func (mc *MetricsCollector) RecordSession(
    metrics *session.SessionMetrics,
    model, stepType string,
    durationMs int64,
    compactions int,        // NEW
    maxContextPct float64,  // NEW
) string {
    // ... existing logic ...
    mc.current.totalCompactions += compactions
    if maxContextPct > mc.current.maxContextPct {
        mc.current.maxContextPct = maxContextPct
    }
}
```

### Файлы для изменения

| Файл | Изменения |
|:---|:---|
| `session/session.go` | Execute: pipe stdout + stream parse; buildArgs: stream-json format |
| `session/stream.go` | **НОВЫЙ**: StreamAccumulator, NDJSON парсинг |
| `session/result.go` | SessionResult: новые поля (CompactionCount, MaxContextFill, ContextFillPct) |
| `runner/metrics.go` | TaskMetrics, RunMetrics: новые поля; RecordSession: новые параметры |
| `runner/runner.go` | execute(): передавать compaction данные в RecordSession |
| `config/config.go` | Возможно: OutputFormat field (для обратной совместимости) |

### Оценка сложности

- **Фаза 1** (структуры данных): ~2 часа, низкий риск
- **Фаза 2** (потоковый парсинг): ~6-8 часов, средний риск (goroutine lifecycle, edge cases)
- **Фаза 3** (buildArgs): ~30 мин, низкий риск
- **Фаза 4** (метрики): ~2-3 часа, низкий риск
- **Тесты**: ~4-6 часов

**Итого**: ~15-20 часов, 2-3 story points

## 4. Analysis of Current Data

### Расчёты для mentorlearnplatform

#### Модель роста контекста

Для типичной execute сессии (27 turns, cache_read 1.4M):
```
Средний cache_read/turn = 1,418,382 / 27 ≈ 52,533 токенов
```

Это означает средний размер кеша на turn ≈ 52K. Но рост нелинеен — ранние turns читают меньше кеша (контекст ещё маленький), поздние — больше.

При линейном росте от 0 до X за N turns:
```
sum = N * X / 2 = total_cache_read
X = 2 * total_cache_read / N = 2 * 1,418,382 / 27 ≈ 105,066 токенов
```

Это означает что к последнему turn'у кешированный контекст ≈ 105K. С учётом non-cached input (65K / 27 ≈ 2.4K per turn), общий context fill на последнем turn'е ≈ 107K.

107K / 200K = **53.5%** — ниже порога компакта (83.5%) и в пределах "качественной зоны" (40-50%).

#### Для "тяжёлой" execute #11 (41 turn, cache_read 3.1M)

```
X_max = 2 * 3,149,106 / 41 ≈ 153,615 токенов (кеш)
+ non-cached: 137,637 / 41 ≈ 3,357 per turn
Total context fill на последнем turn'е ≈ 157K
```

157K / 200K = **78.5%** — ПРИБЛИЖАЕТСЯ к порогу компакта (83.5%). Ещё несколько turns — и будет компакт.

#### Выводы из данных

1. **Execute до 30 turns**: context fill ≤ 55%, безопасная зона
2. **Execute 35-41 turn**: context fill 65-79%, зона риска
3. **Execute 41+ turns**: вероятен компакт на 44-47 turn'е
4. **Review сессии**: context fill ≤ 40%, безопасная зона
5. **max_turns: 50** — при текущей нагрузке, 50 turns может вызвать 1-2 компакта

#### Рекомендация по max_turns

Текущий `max_turns: 50` — агрессивный. По данным mentorlearnplatform, безопасный лимит для execute ≈ 35-40 turns. Для review ≈ 35 turns (уже в рамках текущего поведения).

Однако это ГИПОТЕЗА, основанная на модели линейного роста. Реальный рост может быть нелинейным (например, резкий скачок при чтении больших файлов). Для точных данных нужен Вариант D.

## 5. Risks & Limitations

### Чего мы НЕ МОЖЕМ узнать (и почему)

1. **Точный moment компакта** — `CompactBoundaryMessage` показывает ФАКТ, но не точный turn#. Для этого нужно отслеживать per-turn usage и вычислять, когда fill превысил порог.

2. **Качество после компакта** — компакт суммаризирует историю, неизбежно теряя детали. Мы можем определить СКОЛЬКО компактов было, но не КАК СИЛЬНО это повлияло на качество. По данным community: ~60-70% framework compliance после компакта vs ~95% до.

3. **Содержимое суммаризации** — мы не видим что именно компакт сохранил и что потерял. PostCompact hook не существует.

4. **Реальный per-turn context fill без stream-json** — текущий `--output-format json` даёт только кумулятивные totals. Все расчёты в разделе 4 — ОЦЕНКИ на основе модели линейного роста.

5. **Влияние tool outputs на context fill** — когда Claude читает файл через Read tool, содержимое файла добавляется в контекст. Мы не знаем объём tool outputs per turn без stream-json парсинга.

### Технические риски реализации

1. **Goroutine lifecycle в Execute()** — pipe + goroutine для парсинга NDJSON требует аккуратного управления (закрытие pipe при ошибке, deadlock prevention)

2. **NDJSON edge cases** — большие строки, malformed JSON, неожиданные типы сообщений. Нужен robust scanner с max line size limit.

3. **Обратная совместимость** — если старая версия Claude Code не поддерживает `CompactBoundaryMessage`, парсер не сломается (просто не найдёт маркеры), но метрики будут нулевыми.

4. **Производительность** — потоковый парсинг через pipe добавляет overhead (goroutine scheduling, JSON decode per line). Для типичных сессий (14-41 turn, <3M токенов вывода) это пренебрежимо мало.

5. **Тестирование** — нужны mock'и для NDJSON потока. Текущий `config.ClaudeCommand` + mock binary pattern должен работать, но mock должен выдавать NDJSON вместо одного JSON объекта.

### Альтернативные подходы (отклонённые)

- **JSONL transcript парсинг** (`.claude/projects/.../session.jsonl`) — содержит полную историю с компактами, но: (a) файл принадлежит Claude Code, не Ralph; (b) формат не документирован; (c) путь зависит от внутренней структуры Claude Code.

- **Claude API напрямую** (bypass Claude Code) — даёт полный контроль, но теряет все преимущества Claude Code (tools, permissions, hooks, CLAUDE.md).

- **pause_after_compaction** API feature — позволяет приостановить после компакта и сохранить последние сообщения ([issue #23457](https://github.com/anthropics/claude-code/issues/23457)), но это Claude API feature, не CLI feature.

## Источники

- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [Agent SDK Streaming Output](https://platform.claude.com/docs/en/agent-sdk/streaming-output)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks)
- [StatusLine Documentation](https://code.claude.com/docs/en/statusline)
- [Context Compaction Research (badlogic)](https://gist.github.com/badlogic/cd2ef65b0697c4dbe2d13fbecb0a0a5f)
- [Context Buffer Management](https://claudefa.st/blog/guide/mechanics/context-buffer-management)
- [Calculate Context Usage](https://codelynx.dev/posts/calculate-claude-code-context)
- [Claude Code Agent SDK Spec (Gist)](https://gist.github.com/POWERFULMOVES/58bcadab9483bf5e633e865f131e6c25)
- [PostCompact Hook Issue #14258](https://github.com/anthropics/claude-code/issues/14258)
- [Context Usage Exposure Issue #23457](https://github.com/anthropics/claude-code/issues/23457)
- [AutoCompact Settings Issue #10691](https://github.com/anthropics/claude-code/issues/10691)
- [Run Claude Code Programmatically](https://code.claude.com/docs/en/headless)
