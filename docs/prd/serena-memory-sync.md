# Serena Memory Sync — PRD Shard

**Epic:** 8 — Serena Memory Sync
**Status:** Draft
**Date:** 2026-03-05
**Research:** [docs/research/technical-serena-memory-sync-research.md](../research/technical-serena-memory-sync-research.md)

---

## Контекст

Ralph детектит наличие Serena MCP (FR39) и использует его для token economy в execute-сессиях. Однако Serena memories устаревают по мере выполнения задач: новые типы, интерфейсы, зависимости — всё это остаётся неотражённым до ручного обновления.

Текущая цепочка знаний:
```
execute → review → extract learnings → LEARNINGS.md → distill
```

Serena memories — параллельная, несвязанная система. Конкурентный анализ показывает: ни один инструмент (Aider, SWE-agent, Devin, Cline, Codex CLI) не предлагает автоматическую синхронизацию MCP memory stores. Это уникальное преимущество Ralph.

Текущие пробелы:
- Ralph не инструктирует Claude обновлять Serena memories
- LEARNINGS.md и Serena memories дублируют знания без синхронизации
- После крупного прогона (`ralph run` с 10+ задачами) memories сильно устаревают
- Ручное обновление — рутинная работа разработчика

---

## Scope

Отдельная sync-сессия (Вариант 2 из исследования) — Claude subprocess, сфокусированный исключительно на обновлении memories. Batch по умолчанию (после всех задач), опциональный per-task trigger. Zero new external dependencies.

Out of scope:
- Прямая запись в `.serena/memories/` без Claude (Вариант 3)
- Промпт-инъекция в существующие execute/review сессии (Вариант 1)
- Поддержка MCP memory stores кроме Serena
- Кастомизация содержимого sync через config (Growth)
- Автоматическое создание новых memories (только обновление существующих)

---

## Функциональные требования

### Sync-сессия (Core)

- **FR57:** Система запускает отдельную Serena sync-сессию после завершения всех задач `ralph run` (SYNC POINT C). Условие запуска: наличие Serena MCP (`CodeIndexerDetector.Available()`) И включённая настройка `serena_sync_enabled`. Sync-сессия — свежий Claude subprocess через `session.Execute` с промптом, сфокусированным исключительно на обновлении memories. При выключенной настройке или отсутствии Serena — graceful skip без ошибки

- **FR58:** Sync-сессия получает контекст изменений: (a) `git diff --stat` всего прогона (first commit..HEAD), (b) содержимое LEARNINGS.md, (c) список завершённых задач из sprint-tasks.md. Промпт инструктирует Claude: прочитать текущие memories (`list_memories` + `read_memory`), обновить устаревшие (`edit_memory` предпочтительно, `write_memory` для полной перезаписи), НЕ удалять и НЕ создавать memories без явной необходимости

- **FR59:** Sync-промпт реализован как Go-шаблон в `runner/prompts/serena-sync.md`. Следует существующему паттерну двухэтапной сборки: `text/template` для структуры (`{{if .HasLearnings}}`) + `strings.Replace` для user-контента (`__DIFF_SUMMARY__`, `__LEARNINGS_CONTENT__`, `__COMPLETED_TASKS__`). Промпт не содержит user-controlled `{{` — защита от template injection

- **FR60:** Sync-сессия ограничена по масштабу: `serena_sync_max_turns` (default 5) определяет максимум ходов Claude. При превышении — log warning и завершение. Сессия НЕ использует `--resume` (одноразовая, как distillation)

### Защита данных (Safety)

- **FR61:** Перед запуском sync-сессии система создаёт backup `.serena/memories/` → `.serena/memories.bak/` (копирование всех файлов). При ошибке sync-сессии (exit code != 0) или провале валидации — автоматический rollback из backup. Backup удаляется после успешной валидации

- **FR62:** После sync-сессии система валидирует результат: количество memories не уменьшилось (подсчёт файлов в `.serena/memories/`). При нарушении — rollback из backup + WARNING в лог. Валидация — best effort: при ошибке чтения директории — skip валидации, не блокировать pipeline

### Конфигурация (Config)

- **FR63:** Три новых поля в `config.Config`: `serena_sync_enabled bool` (default false), `serena_sync_max_turns int` (default 5), `serena_sync_trigger string` (default "run", допустимые: "run", "task"). Поля парсятся из `.ralph/config.yaml` с fallback на defaults. CLI флаг `--serena-sync` включает `serena_sync_enabled` без правки config файла

- **FR64 (Growth):** Trigger per-task (`serena_sync_trigger: "task"`) запускает sync-сессию после каждой завершённой задачи (SYNC POINT B, после knowledge extraction). Для крупных прогонов с архитектурными изменениями, где batch sync может быть перегружен контекстом. При per-task trigger backup/rollback выполняется на каждую задачу

### Observability

- **FR65:** Метрики sync-сессии записываются в `RunMetrics`: токены (input/output/cache), стоимость, duration_ms, статус (success/skipped/failed/rollback). В текстовом run summary — строка `Serena sync: success ($0.05, 12s)` или `Serena sync: skipped (disabled)`. В JSON report — секция `serena_sync` с полными метриками

- **FR66:** При недоступности Serena MCP в момент sync (MCP server down, timeout) — graceful skip с `WARN` в лог и `status: "serena_unavailable"` в метриках. Sync failure не влияет на exit code `ralph run` — это best-effort операция

---

## Нефункциональные требования

- **NFR26:** Sync-сессия не блокирует основной pipeline: выполняется после run summary, failure не меняет exit code. Максимальное время sync: `serena_sync_max_turns * средняя длительность хода` (~30-60 секунд)

- **NFR27:** Backup/rollback надёжен на NTFS (WSL): используется `os.CopyFile` / `os.MkdirAll` + `filepath.Walk`, без symlinks. При ошибке копирования — skip sync с warning

- **NFR28:** Новые config поля имеют sensible defaults (отключено по умолчанию). `ralph run` без изменений config работает как раньше. Включение — одна строка в config или один CLI флаг

- **NFR29:** Sync-промпт следует архитектуре Ralph: Go template + strings.Replace, размещён в `runner/prompts/`, интегрирован через `buildTemplateData`

---

## User Stories (высокоуровневые)

### US6: Автоматическое обновление Serena memories
**Как** разработчик, использующий Serena MCP, **я хочу** чтобы ralph автоматически обновлял Serena memories после прогона, **чтобы** мои AI-инструменты всегда имели актуальную карту проекта без ручной работы.
- **AC1:** После `ralph run` с `--serena-sync` memories обновлены (diff в содержимом)
- **AC2:** Новые типы/интерфейсы из кода отражены в architecture memories
- **AC3:** Lessons из LEARNINGS.md отражены в testing/patterns memories
- **AC4:** Статус проекта (завершённые задачи) обновлён

### US7: Безопасность memories при sync
**Как** разработчик, **я хочу** чтобы sync не мог испортить мои memories, **чтобы** я мог включить автосинхронизацию без страха потери данных.
- **AC1:** Backup создаётся перед каждым sync
- **AC2:** При ошибке sync — автоматический rollback
- **AC3:** Количество memories не уменьшается после sync
- **AC4:** Существующие memories не удаляются

### US8: Контроль стоимости sync
**Как** разработчик, **я хочу** контролировать стоимость sync-сессий, **чтобы** автосинхронизация не съедала непропорциональную долю бюджета.
- **AC1:** Sync по умолчанию выключен
- **AC2:** Max turns ограничивает длительность сессии
- **AC3:** Метрики sync видны в run summary
- **AC4:** Batch sync (после всех задач) экономит 5-6x vs per-task

---

## Зависимости

- **Внутренние:** `CodeIndexerDetector` interface (FR39), `session.Execute` (FR7), `buildTemplateData` (FR39), `RunMetrics` (FR42-FR56), `Config` struct (FR30-FR31)
- **Внешние:** Zero new dependencies. Используется Go stdlib (`os`, `filepath`, `io/fs`)
- **Serena MCP:** Требуется доступность memory tools (`list_memories`, `read_memory`, `write_memory`, `edit_memory`)

---

## Риски

| Риск | Вероятность | Impact | Mitigation |
|------|-------------|--------|------------|
| Claude пишет некорректный контент в memories | Средняя | High | Backup/rollback (FR61), валидация count (FR62), restricted промпт |
| Claude удаляет memories несмотря на запрет | Низкая | High | Count валидация (FR62), промпт запрещает удаление (FR58) |
| Конфликт с ручными обновлениями memories | Средняя | Medium | `edit_memory` > `write_memory`, sync после run (не параллельно) |
| Serena MCP API changes (memory tools) | Низкая | Medium | Graceful fallback (FR66), MCP tools = stable contract |
| Превышение стоимости sync на крупных проектах | Низкая | Low | Max turns (FR60), batch по умолчанию, метрики (FR65) |
| NTFS quirks при backup/rollback | Средняя | Low | `filepath.Walk` + explicit copy, WSL-tested (NFR27) |

---

## Приоритеты реализации

| Priority | Items | LOC est. | Dependencies |
|----------|-------|----------|--------------|
| P0 (core) | FR57, FR58, FR59, FR60 | ~400 | FR39 (Serena detection) |
| P1 (safety) | FR61, FR62 | ~200 | P0 (sync session) |
| P2 (config) | FR63 | ~150 | None (extends existing config) |
| P3 (observability) | FR65, FR66 | ~150 | P0 + FR42 (RunMetrics) |
| P4 (growth) | FR64 | ~100 | P0 + P1 |
| **Total** | **10 FRs, 4 NFRs** | **~1000** | **Zero new deps** |
