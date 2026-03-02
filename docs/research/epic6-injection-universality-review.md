# Epic 6 Injection & Universality — Критический обзор (Stories 6.7, 6.8)

**Совместный consensus document**
**Пара 3:** analyst-injection + architect-injection
**Дата:** 2026-02-28
**Scope:** Stories 6.7 (Code Indexer Integration), 6.8 (Multi-File Knowledge Tests)
**Восстановлено из:** сводка сессии (оригинал передан через SendMessage, файл не был создан)

---

## Executive Summary

Injection & universality review (Stories 6.7-6.8) выявил **1 CRITICAL, 3 HIGH и 3 MEDIUM** проблем. Ключевая: Story 6.7 целиком основана на несуществующем CLI-интерфейсе Serena (`serena index --full`), хотя Serena — MCP-сервер, а не CLI-утилита. Без исправления Story 6.7 нереализуема.

Обзор подтверждает: решения по универсальности (динамические scope hints, язык-независимые цитаты) — правильные, но требуют доработки в нескольких точках.

---

## Issue 1 (CRITICAL): Serena = MCP server, а не CLI

### Описание проблемы

Story 6.7 описывает интеграцию с Serena через CLI:
- `exec.LookPath("serena")` для детекции
- `serena index --full` для индексации
- `serena query <symbol>` для запросов
- Fallback на grep при отсутствии Serena

**Проблема:** Serena (https://github.com/mcp-sh/serena) — MCP-сервер, который работает через JSON-RPC протокол MCP. У Serena **нет CLI-интерфейса**. Команды `serena index --full` и `serena query` **не существуют**.

Детекция через `exec.LookPath("serena")` бессмысленна — Serena запускается через конфигурацию MCP-клиента (Claude Code), а не как standalone binary.

### Импакт

- Вся Story 6.7 нереализуема в текущем виде
- 11 AC в Story 6.7 основаны на несуществующем API
- Тесты (Story 6.8 AC, зависящие от 6.7) тоже невалидны

### Консенсус: Переписать как MCP-based integration

**Детекция Serena:**
- Проверить `.claude/settings.json` или `.mcp.json` на наличие Serena MCP server config
- НЕ `exec.LookPath`, а чтение конфигурационного файла

**Взаимодействие:**
- Ralph НЕ вызывает Serena напрямую
- Ralph включает в prompt инструкции: "If Serena MCP tools are available, use them to find related code context"
- Claude внутри сессии сам решает, доступна ли Serena и использует MCP tools

**CodeIndexer interface:**
- Абстракция `CodeIndexer` остаётся правильной (для будущих альтернатив)
- Но реализация = prompt-based guidance, не Go exec calls

### Требуемые изменения в AC

- Story 6.7: полная переработка AC — убрать все exec.Command/LookPath паттерны
- Story 6.7: детекция через MCP config file, не binary lookup
- Story 6.7: interaction через prompt instructions, не Go API calls
- Story 6.8: обновить тесты для MCP-based integration

---

## Issue 2 (HIGH): Cross-language test coverage отсутствует

### Описание проблемы

Story 6.8 (Multi-File Knowledge Tests) не определяет тестовые сценарии для не-Go проектов. Все примеры в AC подразумевают Go-проект:
- `*_test.go` file patterns
- Go-style error wrapping в примерах
- `doc comments` в scope hints

bmad-ralph — универсальный CLI. Тесты должны покрывать Python, JS/TS, Rust и другие проекты.

### Консенсус: Добавить cross-language test scenarios

- Минимум 2 языковых сценария в тестах: Go + один non-Go (Python или JS/TS)
- Тестировать: scope hint detection для `.py`, `.ts`, `.rs` файлов
- Тестировать: citation format с non-Go file extensions
- Тестировать: category names не привязаны к Go-специфичным терминам

### Требуемые изменения в AC

- Story 6.8: добавить test scenario "multi-language project" с mixed file types
- Story 6.8: добавить test scenario "non-Go scope hints detection"

---

## Issue 3 (HIGH): Scope hints detection — неясный алгоритм

### Описание проблемы

Story 6.5 AC: "scope hints auto-detected from project file types." Нет определения:
1. Как именно Go-код определяет file types (glob по extensions? top-level только?)
2. Что делать с monorepo (Go + Python + JS)?
3. Маппинг file type → glob pattern (`.py` → `**/*.py`? `*.py`?)
4. Что если проект не имеет стандартных extensions (Makefile, Dockerfile)?

### Консенсус: Explicit detection algorithm в AC

**Алгоритм:**
1. Walk top 2 levels of project root
2. Collect unique file extensions
3. Map to known language globs (table in Go code)
4. Monorepo: ALL detected languages → combined scope hints
5. Unknown extensions → no scope hint (catch-all ralph-misc.md)

**Known mapping (examples):**
| Extension | Scope hint glob |
|-----------|----------------|
| `.go` | `["*.go", "**/*.go"]` |
| `.py` | `["*.py", "**/*.py"]` |
| `.ts`, `.tsx` | `["*.ts", "*.tsx", "**/*.ts", "**/*.tsx"]` |
| `.rs` | `["*.rs", "**/*.rs"]` |

### Требуемые изменения в AC

- Story 6.5 Technical Notes: определить detection algorithm
- Story 6.5 Technical Notes: определить known extension → glob mapping
- Story 6.5 AC: "scope hints cover ALL detected project languages"

---

## Issue 4 (HIGH): Category drift для non-Go проектов

### Описание проблемы

Стандартизированный список категорий (из Pair 1 consensus) ориентирован на Go:
`testing, errors, config, cli, architecture, performance, documentation`

Проблемы для других языков:
- Python-проект может генерировать `imports`, `async`, `packaging` — не в стандартном списке
- JS/TS: `types`, `bundling`, `hooks` — специфичные категории
- Жёсткий список отклоняет валидные entries из non-Go проектов

### Консенсус: Core + extension categories

**Core categories (universal, всегда доступны):**
`testing, errors, config, architecture, performance, documentation, security`

**Extension mechanism:**
- Prompt: "Use core categories when possible. If a lesson doesn't fit any core category, use a descriptive 1-2 word category."
- Go dedup: normalize ALL categories (core + custom) одинаково
- Distillation: может предложить новые canonical categories на основе custom usage patterns

### Требуемые изменения в AC

- Story 6.1: стандартизированный список = core categories (universal)
- Story 6.1 Technical Notes: extension mechanism описание
- Story 6.3/6.4 prompts: core + extension instructions

---

## Issue 5 (MEDIUM): CodeIndexer interface — over-engineering для v1

### Описание проблемы

Story 6.7 определяет полный `CodeIndexer` interface с `Index()`, `Query()`, `Available()`. При MCP-based подходе (Issue 1 fix) Go-код не вызывает indexer напрямую — только детектирует availability.

Full interface = мёртвый код в v1. Нарушает YAGNI.

### Консенсус: Minimal interface для v1

```go
type CodeIndexerDetector interface {
    Available(projectRoot string) bool  // checks MCP config
    PromptHint() string                 // returns prompt instruction text
}
```

Полный `CodeIndexer` с `Index()`/`Query()` — deferred to Growth phase.

### Требуемые изменения в AC

- Story 6.7: упростить interface до detection + prompt hint
- Story 6.7: убрать Index() и Query() из AC (Growth phase)

---

## Issue 6 (MEDIUM): Injection circuit breaker threshold не обоснован

### Описание проблемы

"Stop injecting LEARNINGS.md at 3x budget (600 lines)" — откуда 3x? При 200-line budget:
- 600 lines = ~6% context window (при 128K tokens)
- Исследования (R1 [S5]) показывают деградацию при >10% context pollution
- 3x — произвольный множитель без research backing

### Консенсус: 3x допустим, но нужен config field

3x (600 lines при 200-line budget) = ~6% context, что ниже 10% degradation threshold. Допустимо.

Но threshold должен быть:
1. Named constant в Go коде (не magic number)
2. Config field для override: `injection_budget_multiplier: 3` (default)

### Требуемые изменения в AC

- Story 6.2 Technical Notes: обосновать 3x threshold (6% < 10% threshold)
- Story 6.2 AC: named constant для multiplier

---

## Issue 7 (MEDIUM): Self-healing при circuit breaker — неясная trigger condition

### Описание проблемы

"Self-healing on distillation" — при успешной distillation circuit breaker resets и injection возобновляется. Но:
- Что если distillation уменьшила размер с 600 до 180 lines? Всё ещё < 3x budget
- Что если distillation уменьшила только до 550 lines? Всё ещё > budget
- Trigger для self-healing: DistillState.IsOpen=false ИЛИ lines < threshold?

### Консенсус: Lines-based, не state-based

Self-healing condition: `currentLines < injectionThreshold`, не `CB.IsOpen == false`.
Distillation может fail (CB stays OPEN), но если lines уменьшились через manual edit → injection возобновляется.

### Требуемые изменения в AC

- Story 6.2 AC: уточнить self-healing trigger = `lines < threshold`, не CB state

---

## Сводная таблица

| # | Severity | Issue | Consensus Decision | Stories Affected |
|---|----------|-------|--------------------|--------------------|
| 1 | **CRITICAL** | Serena = MCP, не CLI | Переписать как MCP-based integration | 6.7, 6.8 |
| 2 | **HIGH** | Cross-language tests отсутствуют | Добавить multi-language test scenarios | 6.8 |
| 3 | **HIGH** | Scope hints detection не определён | Explicit detection algorithm в AC | 6.5, 6.8 |
| 4 | **HIGH** | Category drift для non-Go | Core + extension categories | 6.1, 6.3, 6.4 |
| 5 | MEDIUM | CodeIndexer over-engineering | Minimal interface (detection + hint) | 6.7 |
| 6 | MEDIUM | Injection CB threshold не обоснован | 3x допустим, добавить config field | 6.2 |
| 7 | MEDIUM | Self-healing trigger неясен | Lines-based, не state-based | 6.2 |

---

## Позитивные аспекты (что НЕ нужно менять)

1. **Dynamic scope hints** — правильное решение для универсальности (уже исправлено в сессии)
2. **Citation format `file:line`** — universal, не Go-специфичный (уже исправлено)
3. **CodeIndexer abstraction** — правильный подход, только simplify для v1
4. **Fallback на grep** — правильный при отсутствии code indexer
5. **Multi-file ralph-{category}.md** — правильная архитектура для progressive disclosure

---

*Документ восстановлен из сводки предыдущей сессии. Оригинальные findings были переданы через SendMessage и вошли в итоговую сводку, но не были записаны в файл агентами.*
