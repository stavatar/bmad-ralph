# Техническое исследование: автоматическая синхронизация Serena memories в Ralph

**Дата:** 2026-03-05
**Автор:** Степан
**Тип исследования:** Technical Research
**Актуальность данных:** 2026

---

## Executive Summary

Ralph запускает Claude Code subprocess, который имеет доступ к Serena MCP tools (включая `write_memory`, `edit_memory`, `read_memory`). Однако Ralph **не инструктирует** Claude обновлять Serena memories — это побочный эффект наличия Serena в `.mcp.json`. Исследование анализирует варианты автоматической синхронизации знаний Ralph (LEARNINGS.md, архитектурные изменения, метрики) с Serena memories через prompt injection в Claude subprocess.

**Ключевые выводы:**

- Serena memories — файлы `.serena/memories/*.md`, читаемые/записываемые через MCP tools
- Ralph уже детектит Serena через `runner/serena.go` (`CodeIndexerDetector` interface)
- Интеграция через промпт-инструкции в `--append-system-prompt` — самый чистый подход
- Отдельная «sync-сессия» после execute/review цикла — оптимальная точка интеграции
- Риски: порча memories некорректным контентом, дополнительная стоимость токенов

**Рекомендации:**

1. Новый промпт-шаблон `runner/prompts/serena-sync.md` с инструкциями обновления memories
2. Отдельная sync-сессия (короткая, `--max-turns 5`) после завершения задачи
3. Условная активация: только при `SerenaEnabled && serena_sync_enabled`
4. Валидация: сравнение memories до/после, rollback при ошибке

---

## Содержание

1. [Serena Memory System — как это работает](#1-serena-memory-system)
2. [Текущая интеграция Ralph ↔ Serena](#2-текущая-интеграция)
3. [Точки интеграции в цикле Ralph](#3-точки-интеграции)
4. [Архитектурные варианты](#4-архитектурные-варианты)
5. [Рекомендуемый подход](#5-рекомендуемый-подход)
6. [Стоимость и риски](#6-стоимость-и-риски)
7. [Выводы и рекомендации для PRD](#7-выводы-и-рекомендации)

---

## 1. Serena Memory System

### 1.1 Хранилище

Serena memories — обычные markdown-файлы в `.serena/memories/` директории проекта. Каждый файл — отдельная «memory» с именем, которое может включать `/` для организации по темам (например `architecture/runner_symbols`).

_Источник: [Serena Documentation — Configuration](https://oraios.github.io/serena/02-usage/050_configuration.html)_

### 1.2 MCP Tools API

| Tool | Параметры | Назначение |
|------|-----------|------------|
| `write_memory` | `memory_name`, `content`, `max_chars` | Создать/перезаписать memory целиком |
| `edit_memory` | `memory_name`, `needle`, `repl`, `mode` | Поиск/замена в существующей memory (literal или regex) |
| `read_memory` | `memory_name` | Прочитать содержимое memory |
| `list_memories` | `topic` (optional) | Список доступных memories с фильтрацией по теме |
| `delete_memory` | `memory_name` | Удалить memory |
| `rename_memory` | `old_name`, `new_name` | Переименовать/переместить memory |

_Источник: [Serena Tools — Glama](https://glama.ai/mcp/servers/@oraios/serena/tools/write_memory), [Serena Documentation — Tools](https://oraios.github.io/serena/01-about/035_tools.html)_

### 1.3 Onboarding и начальное заполнение

При первом запуске Serena анализирует структуру проекта и создаёт начальные memories. Инструмент `check_onboarding_performed` проверяет, было ли выполнено онбординг. Инструмент `onboarding` запускает процесс создания начальной базы знаний.

_Источник: [Serena MCP — LobeHub](https://lobehub.com/mcp/oraios-serena)_

### 1.4 Ключевые характеристики

- **Персистентность:** файлы на диске, переживают перезапуск
- **Формат:** UTF-8 markdown, свободная структура
- **Организация:** иерархическая через `/` в именах (topics)
- **Размер:** контролируется `max_chars` параметром (default из config)
- **Доступ:** read/write через MCP tools из Claude subprocess
- **Актуальность:** не обновляются автоматически при изменении кода

[High Confidence] — подтверждено из документации и практического использования в проекте bmad-ralph.

---

## 2. Текущая интеграция

### 2.1 Serena Detection в Ralph

Ralph уже детектит наличие Serena MCP через `runner/serena.go`:

```go
type CodeIndexerDetector interface {
    Available(projectRoot string) bool
    PromptHint() string
}
```

Реализации:
- `SerenaMCPDetector` — проверяет `.mcp.json` на наличие serena конфигурации
- `NoOpCodeIndexerDetector` — заглушка (Serena отсутствует)

`DetectSerena()` возвращает prompt hint, который вставляется в `TemplateData.SerenaEnabled`.

### 2.2 Текущее использование в промптах

В `runner/prompts/execute.md` есть условный блок:

```
{{if .SerenaEnabled}}
Serena MCP code indexing is available...
{{end}}
```

Это лишь **информирует** Claude о наличии Serena. Ralph не инструктирует Claude обновлять memories.

### 2.3 Текущий Knowledge Pipeline

```
execute → review → extract learnings → LEARNINGS.md → distill (при превышении budget)
```

LEARNINGS.md — собственная система Ralph. Serena memories — отдельная, несвязанная система.

---

## 3. Точки интеграции

### 3.1 Карта цикла Ralph с потенциальными точками sync

```
┌─────────────────────────────────────────────────────┐
│ ralph run                                            │
│                                                      │
│  ┌── Задача N ──────────────────────────────────┐   │
│  │                                                │   │
│  │  1. Execute session  ←── [SYNC POINT A]       │   │
│  │  2. Git diff + metrics                         │   │
│  │  3. Review session                             │   │
│  │  4. Knowledge extraction → LEARNINGS.md        │   │
│  │  5. ───────────────────── [SYNC POINT B] ──── │   │
│  │  6. Gate (если включён)                        │   │
│  │                                                │   │
│  └────────────────────────────────────────────────┘   │
│                                                      │
│  ┌── После всех задач ──────────────────────────┐   │
│  │  7. Run summary                                │   │
│  │  8. ───────────────────── [SYNC POINT C] ──── │   │
│  └────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### 3.2 Анализ точек

| Точка | Когда | Что синхронизировать | Плюсы | Минусы |
|-------|-------|---------------------|-------|--------|
| **A** | После execute | Архитектурные изменения (новые типы, интерфейсы) | Свежая информация | Может быть отвергнуто ревью |
| **B** | После knowledge extraction | LEARNINGS + архитектура | Проверенные изменения | Дополнительная сессия на каждую задачу |
| **C** | После всех задач | Всё сразу (batch) | Одна сессия, экономия токенов | Может быть слишком много изменений |

### 3.3 Рекомендация: SYNC POINT C (batch после всех задач)

- **Экономия:** одна сессия вместо N
- **Качество:** все изменения проверены ревью
- **Простота:** не усложняет основной цикл
- **Fallback:** SYNC POINT B как опция для крупных прогонов

---

## 4. Архитектурные варианты

### Вариант 1: Промпт-инъекция в существующую сессию

**Суть:** Добавить инструкции обновления memories прямо в execute/review промпт.

```
{{if .SerenaSyncEnabled}}
After completing the task, update Serena memories...
{{end}}
```

**Плюсы:** Нет дополнительных сессий, нулевая стоимость.
**Минусы:** Claude может проигнорировать побочную инструкцию. Конфликт приоритетов — основная задача vs. обновление memories. Нет контроля результата.

**Оценка:** ❌ Ненадёжно.

### Вариант 2: Отдельная sync-сессия

**Суть:** Запускать отдельную короткую Claude сессию (`--max-turns 5`) с промптом, сфокусированным исключительно на обновлении memories.

```go
// В runner.go, после основного цикла:
if cfg.SerenaSyncEnabled && serenaHint != "" {
    r.runSerenaSync(ctx, runMetrics)
}
```

**Промпт:** Содержит:
- Список изменённых файлов (из git diff)
- Извлечённые LEARNINGS
- Инструкции: прочитать текущие memories → обновить → записать

**Плюсы:** Надёжно — единственная задача Claude. Контролируемый результат. Можно валидировать.
**Минусы:** Дополнительные токены (~5-15K на sync). Дополнительная latency.

**Оценка:** ✅ Рекомендуется.

### Вариант 3: Прямая запись в файлы (.serena/memories/)

**Суть:** Ralph напрямую пишет в `.serena/memories/` без Claude, используя шаблоны.

**Плюсы:** Нулевая стоимость. Мгновенно. Детерминистично.
**Минусы:** Потеря «интеллекта» — Ralph не понимает семантику memories. Шаблонное обновление не может адаптироваться к контексту. Нарушение абстракции Serena (файлы — implementation detail).

**Оценка:** ⚠️ Можно для простых случаев (метрики, статус), но не для семантического обновления.

### Вариант 4: Гибрид (Вариант 3 + Вариант 2)

**Суть:** Простые обновления (статус, метрики) — прямая запись. Семантические (архитектура, learnings) — через Claude сессию.

**Плюсы:** Баланс стоимости и качества.
**Минусы:** Два механизма — сложнее поддерживать.

**Оценка:** ⚠️ Over-engineering для MVP.

---

## 5. Рекомендуемый подход

### 5.1 Архитектура: Вариант 2 (отдельная sync-сессия)

```
ralph run
  ├── [основной цикл: execute → review → knowledge]
  ├── Run Summary
  └── Serena Sync Session (если SerenaEnabled && SerenaSyncEnabled)
        ├── Читает: git diff --stat, LEARNINGS.md, текущие memories
        ├── Claude: анализирует изменения, обновляет memories
        └── Валидация: проверка что memories не испорчены
```

### 5.2 Новые компоненты

| Компонент | Пакет | Назначение |
|-----------|-------|------------|
| `serena-sync.md` | `runner/prompts/` | Go-шаблон промпта sync-сессии |
| `SerenaSyncConfig` | `config/` | Поля: `serena_sync_enabled`, `serena_sync_max_turns` |
| `runSerenaSync()` | `runner/` | Метод Runner — запуск sync-сессии |
| `SerenaSyncEnabled` | `config/TemplateData` | Boolean для условного блока в промпте |

### 5.3 Промпт sync-сессии (концепция)

```markdown
# Serena Memory Sync

Ты — агент синхронизации знаний. Твоя единственная задача — обновить
Serena memories проекта на основе изменений последнего прогона Ralph.

## Контекст изменений

__DIFF_SUMMARY__

## Извлечённые уроки

__LEARNINGS_CONTENT__

## Инструкции

1. Используй `list_memories` чтобы увидеть текущие memories
2. Используй `read_memory` для чтения тех, которые затронуты изменениями
3. Используй `edit_memory` для точечных обновлений (предпочтительно)
4. Используй `write_memory` только для полной перезаписи
5. НЕ удаляй memories
6. НЕ создавай новые memories без явной необходимости
7. Обновляй ТОЛЬКО те memories, информация в которых устарела

## Что обновлять

- Символьная карта пакетов (новые/удалённые типы, интерфейсы, функции)
- Зависимости пакетов (новые импорты, изменённые связи)
- Статус проекта (завершённые задачи, метрики)
- Паттерны и конвенции (новые уроки из LEARNINGS.md)
```

### 5.4 Интеграция в dependency graph

```
runner/runner.go  →  runSerenaSync()  →  session.Execute()
                                           ↑
                     runner/prompts/serena-sync.md (template)
                                           ↑
                     config.TemplateData.SerenaSyncEnabled
```

Направление зависимостей сохраняется: `runner → session, config`. Новый код только в `runner/`.

### 5.5 Конфигурация

```yaml
# .ralph/config.yaml
serena_sync_enabled: false    # По умолчанию выключено
serena_sync_max_turns: 5      # Короткая сессия
serena_sync_trigger: "run"    # "run" = после ralph run, "task" = после каждой задачи
```

### 5.6 Двухэтапная сборка промпта

Следует существующему паттерну:
1. `text/template` — условные блоки (`{{if .HasLearnings}}`)
2. `strings.Replace` — user-контент (`__DIFF_SUMMARY__`, `__LEARNINGS_CONTENT__`)

---

## 6. Стоимость и риски

### 6.1 Стоимость токенов

| Сценарий | Input tokens | Output tokens | Стоимость (Sonnet) |
|----------|-------------|---------------|-------------------|
| Sync после 5 задач | ~10K | ~5K | ~$0.05 |
| Sync после 10 задач | ~15K | ~8K | ~$0.08 |
| Sync после каждой задачи (×10) | ~50K | ~30K | ~$0.30 |

[Medium Confidence] — оценки на основе типичного размера memories и diff.

**Вывод:** Batch sync (SYNC POINT C) экономит 5-6x по сравнению с per-task sync.

### 6.2 Риски

| Риск | Вероятность | Импакт | Митигация |
|------|-------------|--------|-----------|
| Порча memories (Claude пишет некорректный контент) | Средняя | Высокий | Backup перед sync, rollback при ошибке |
| Удаление memories | Низкая | Высокий | Промпт запрещает удаление, валидация count |
| Конфликт с ручными обновлениями | Средняя | Средний | Merge-стратегия (edit_memory > write_memory) |
| Превышение бюджета sync-сессии | Низкая | Низкий | `--max-turns 5`, таймаут |
| Serena MCP недоступен | Низкая | Низкий | Graceful skip, логирование |

### 6.3 Митигация: backup и rollback

```go
// Перед sync:
backupMemories(projectRoot)  // копия .serena/memories/ → .serena/memories.bak/

// После sync:
if validateMemories(projectRoot) != nil {
    rollbackMemories(projectRoot)  // восстановление из backup
}
```

---

## 7. Выводы и рекомендации для PRD

### 7.1 Функциональные требования (для PRD)

| FR | Описание | Приоритет |
|----|----------|-----------|
| FR-SYNC-1 | Ralph запускает Serena sync-сессию после завершения `ralph run` (при наличии Serena и включённой настройке) | MUST |
| FR-SYNC-2 | Sync-сессия обновляет memories на основе git diff и LEARNINGS.md | MUST |
| FR-SYNC-3 | Backup memories перед sync, rollback при ошибке | MUST |
| FR-SYNC-4 | Конфигурация: `serena_sync_enabled`, `serena_sync_max_turns`, `serena_sync_trigger` | MUST |
| FR-SYNC-5 | Sync-промпт как Go-шаблон в `runner/prompts/serena-sync.md` | MUST |
| FR-SYNC-6 | Метрики sync-сессии (токены, стоимость, latency) записываются в RunMetrics | SHOULD |
| FR-SYNC-7 | Trigger per-task (`serena_sync_trigger: "task"`) для крупных прогонов | COULD |
| FR-SYNC-8 | CLI флаг `--serena-sync` для включения без config файла | SHOULD |
| FR-SYNC-9 | Валидация: количество memories не уменьшилось после sync | SHOULD |
| FR-SYNC-10 | Graceful skip при недоступности Serena MCP | MUST |

### 7.2 Архитектурные решения

1. **Отдельная sync-сессия** (не промпт-инъекция) — надёжность
2. **Batch по умолчанию** (SYNC POINT C) — экономия
3. **Опциональная per-task** (SYNC POINT B) — для крупных прогонов
4. **Backup/rollback** — защита от порчи
5. **Следование паттернам Ralph:** двухэтапный промпт, injectable functions, nil-safe опциональность

### 7.3 Оценка трудозатрат

| Story | Описание | AC (оценка) |
|-------|----------|-------------|
| 8.1 | Sync конфигурация + CLI flags | 4 AC |
| 8.2 | Sync промпт-шаблон | 5 AC |
| 8.3 | Backup/rollback memories | 4 AC |
| 8.4 | runSerenaSync() + интеграция в runner | 6 AC |
| 8.5 | Метрики + валидация | 4 AC |
| 8.6 | Per-task trigger (optional) | 3 AC |
| 8.7 | Интеграционные тесты | 5 AC |

**Итого:** ~7 stories, ~31 AC — сопоставимо с Epic 5 (Human Gates, 6 stories).

---

## Источники

- [Serena GitHub — oraios/serena](https://github.com/oraios/serena)
- [Serena Documentation — Tools](https://oraios.github.io/serena/01-about/035_tools.html)
- [Serena Documentation — Configuration](https://oraios.github.io/serena/02-usage/050_configuration.html)
- [Serena MCP — LobeHub](https://lobehub.com/mcp/oraios-serena)
- [Serena write_memory — Glama](https://glama.ai/mcp/servers/@oraios/serena/tools/write_memory)
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [Claude Code MCP Documentation](https://code.claude.com/docs/en/mcp)
- [--append-system-prompt discussion — GitHub](https://github.com/anthropics/claude-code/issues/6973)
- [MCP Guide 2026](https://www.buildmvpfast.com/blog/model-context-protocol-mcp-guide-2026)
- [Claude Code Subprocess Token Cost](https://dev.to/jungjaehoon/why-claude-code-subagents-waste-50k-tokens-per-turn-and-how-to-fix-it-41ma)

---

**Дата завершения:** 2026-03-05
**Уровень уверенности:** Высокий — основано на документации Serena, коде Ralph и практическом опыте
