# Сравнение 4 вариантов эволюции Ralph Bridge (v2)

**Дата:** 2026-03-07
**Контекст:** ralph v1 реализован (10 эпиков, FR1-FR92). Планирование ralph v2 — переосмысление pipeline планирования задач.

---

## Сводная таблица

| Критерий | A: Bridge упрощается | B: Runner + Stories | C: Runner + Epics | D: ralph plan |
|----------|---------------------|--------------------|--------------------|---------------|
| **Архитектура** | bridge/ с Go-парсером вместо LLM | runner/ расширен, bridge/ удалён | runner/ расширен, bridge/ удалён | planner/ новый пакет, bridge/ deprecated |
| **Входные данные** | BMad story файлы (*.md) | BMad story файлы (*.md) | BMad epic файлы (*.md) | Любой текст: PRD, Issues, plain text |
| **Выходные данные** | sprint-tasks.md (как сейчас) | Внутренний state (`.ralph-state.yaml`) | Внутренний state | sprint-tasks.md (как сейчас) |
| **Зависимость от BMad** | Полная | Полная | Полная | Нулевая (BMad опционален) |
| **Зависимость от LLM (planning)** | Нулевая | Нулевая (парсинг) | Нулевая (парсинг) | Частичная (сложные FR) |
| **Зависимость от формата** | Жёсткая (story с AC) | Жёсткая (story с AC) | Жёсткая (epic формат) | Гибкая (любой markdown) |
| **Объём реализации (LOC)** | ~300 нового + ~400 удаления | ~800 нового + ~2800 удаления | ~1200 нового + ~2800 удаления | ~120 нового + ~2800 удаления |
| **Stories для реализации** | 2-3 | 4-6 | 6-8 | 3-5 |
| **Дни** | 1-2 | 3-5 | 5-7 | 2-3 |
| **Качество декомпозиции** | 6/10 | 8/10 | 4/10 | 7/10 |
| **Time-to-first-task** | 50-120 мин (BMad setup) | 45-110 мин (BMad setup) | 40-100 мин (BMad setup) | 2-5 мин |
| **Стоимость за проект** | $0 (planning) + $2-8 (BMad) | $0 (planning) + $2-8 (BMad) | $0 (planning) + $1-4 (BMad) | $0.10-0.30 (planning) |
| **Гибкость (новые форматы)** | 1/10 | 1/10 | 1/10 | 9/10 |
| **Backward compatibility** | Полная | Нет (runner API меняется) | Нет (весь workflow меняется) | Полная (runner без изменений) |

---

## Вариант A: Bridge остаётся, упрощается

### Суть

Убрать LLM из bridge, заменить программным Go-парсером. Bridge продолжает читать BMad story файлы, но вместо отправки в Claude парсит AC программно и генерирует sprint-tasks.md детерминистически.

### 1. Архитектура

```
cmd/ralph/
├── bridge.go          # CLI (без изменений)
bridge/
├── bridge.go          # Run() — программный парсинг вместо session.Execute()
├── parser.go          # ParseStoryAC() — извлечение AC из markdown
├── classifier.go      # ClassifyAC() — keyword-based классификация
├── grouper.go         # GroupACsToTasks() — эвристическая группировка
├── formatter.go       # FormatTasks() — генерация sprint-tasks.md
├── prompts/
│   └── bridge.md      # УДАЛЯЕТСЯ (244 строки)
```

**Пакеты:** `bridge/` (рефакторинг), `cmd/ralph/bridge.go` (без изменений).
**Промпты:** Удалены полностью. Программный парсинг не требует LLM.
**Команды:** `ralph bridge` (без изменений в CLI-интерфейсе).

### 2. Входные данные

- BMad story файлы (строго `docs/sprint-artifacts/*.md`)
- Формат: markdown с секцией `## Acceptance Criteria` и нумерованными AC
- Без BMad stories — bridge неработоспособен
- Merge mode: существующий sprint-tasks.md (как сейчас)

### 3. Выходные данные

- `sprint-tasks.md` — тот же формат `- [ ]` / `- [x]` с `source:` полями
- Формат идентичен текущему — runner не требует изменений
- Детерминистический: одни и те же stories всегда дают одинаковый результат

### 4. Зависимости

| Зависимость | Статус |
|-------------|--------|
| BMad Method | **Полная** — stories обязательны |
| LLM (planning) | **Нулевая** — программный парсинг |
| LLM (execution) | Без изменений (runner) |
| Формат story | **Жёсткая** — regex парсит конкретную структуру AC |
| sprint-tasks.md формат | Без изменений |

### 5. Объём реализации

| Компонент | LOC | Описание |
|-----------|-----|----------|
| `parser.go` | ~80 | Regex-извлечение AC из markdown |
| `classifier.go` | ~60 | Keyword-based классификация (4 типа AC) |
| `grouper.go` | ~100 | Эвристическая группировка AC в задачи |
| `formatter.go` | ~60 | Генерация sprint-tasks.md строк |
| Рефакторинг `bridge.go` | ~50 (нетто) | Замена session.Execute() на программный pipeline |
| Тесты | ~600 | Unit-тесты для каждого компонента |
| **Удаление** | -400 | bridge.md (244) + session-related код (~156) |
| **Итого нового** | ~300 | + ~600 тестов |

**Stories:** 2-3 (парсер, классификатор+группировщик, тесты).
**Дни:** 1-2 при текущей скорости (3.3 findings/story).

### 6. Качество декомпозиции: 6/10

**Сильные стороны:**
- Детерминизм — одинаковый результат при каждом запуске
- Быстрота — мгновенное выполнение, нет Claude call
- Тестируемость — каждый шаг верифицируем unit-тестами

**Слабые стороны:**
- **Грубая группировка:** 1 AC = 1 task (без семантического "unit of work"). Keyword-эвристики ("same file", "same function") покрывают ~60% случаев, остальные 40% — либо over-decomposed (каждый AC отдельно), либо under-decomposed (всё в одну задачу)
- **Нет понимания контекста:** парсер не знает codebase, не может группировать по файлам/модулям
- **Хрупкость:** любое изменение формата story ломает regex'ы

### 7. Скорость (time-to-first-task)

- BMad pipeline: 50-120 мин (PRD + Architecture + Epics + Stories + Validation)
- Bridge: **<1 сек** (программный парсинг)
- Runner: 5-15 мин
- **Итого: 55-135 мин** (bottleneck = BMad, не bridge)

Bridge перестаёт быть bottleneck, но BMad pipeline остаётся. Экономия: 10-30 мин (время bridge LLM-вызовов).

### 8. Стоимость

- Planning (bridge): **$0** (нет Claude calls)
- BMad setup: $2-8 за проект (stories creation/validation)
- **Итого за проект: $2-8** (экономия $0.60-3.00 на bridge calls)

### 9. Гибкость: 1/10

- Работает ТОЛЬКО с BMad story файлами
- Regex парсинг привязан к конкретной структуре AC
- Любой новый формат (GitHub Issues, plain text, Jira) требует нового парсера
- Нулевая гибкость для новых проектов без BMad

### 10. Backward compatibility: Полная

- CLI-интерфейс `ralph bridge` без изменений
- sprint-tasks.md формат без изменений
- runner без изменений
- Единственное изменение: результат детерминистический (может отличаться от LLM-генерированного)

---

## Вариант B: Bridge убирается, runner работает со stories напрямую

### Суть

Убрать bridge целиком. Runner напрямую читает story файлы, каждая story = scope одной или нескольких Claude-сессий. sprint-tasks.md как промежуточный артефакт исчезает.

### 1. Архитектура

```
cmd/ralph/
├── run.go             # Модифицирован: --stories вместо sprint-tasks.md
runner/
├── scan.go            # ПЕРЕПИСАН: ScanStories() вместо ScanTasks()
├── story_parser.go    # ParseStory() — извлечение AC, Dev Notes
├── progress.go        # ProgressTracker — .ralph-state.yaml
├── runner.go          # Execute() модифицирован: story-based loop
├── prompts/
│   └── execute.md     # Модифицирован: story context вместо task reference
bridge/
  # УДАЛЁН ПОЛНОСТЬЮ
```

**Пакеты:** `runner/` (значительные изменения), `bridge/` (удалён).
**Промпты:** `execute.md` модифицирован — получает полную story, а не ссылку на sprint-tasks.md.
**Команды:** `ralph run` (изменён), `ralph bridge` (удалён).

### 2. Входные данные

- BMad story файлы (строго `docs/sprint-artifacts/*.md`)
- Формат: markdown с секциями AC, Dev Notes, Testing Standards, References
- Story файл подаётся целиком в промпт Claude
- Без BMad stories — ralph неработоспособен

### 3. Выходные данные

- Код (как сейчас)
- `.ralph-state.yaml` — внутренний файл прогресса:
  ```yaml
  stories:
    9-1-progressive-review.md:
      status: done
      completed_at: 2026-03-06T14:30:00Z
    9-2-severity-filtering.md:
      status: in-progress
      current_ac: 3
  ```
- Нет sprint-tasks.md

### 4. Зависимости

| Зависимость | Статус |
|-------------|--------|
| BMad Method | **Полная** — stories обязательны |
| LLM (planning) | **Нулевая** — нет декомпозиции |
| LLM (execution) | Без изменений, но Claude видит ПОЛНУЮ story |
| Формат story | **Жёсткая** — runner парсит конкретную структуру |
| sprint-tasks.md формат | **Не используется** |

### 5. Объём реализации

| Компонент | LOC | Описание |
|-----------|-----|----------|
| `story_parser.go` | ~150 | Парсинг story: AC, Dev Notes, Testing Standards |
| `progress.go` | ~200 | YAML state management, AC-level tracking |
| Рефакторинг `scan.go` | ~100 | ScanStories() вместо ScanTasks() |
| Рефакторинг `runner.go` | ~200 | Story-based loop, partial progress |
| Рефакторинг `execute.md` | ~50 | Story context injection |
| Тесты | ~1500 | Unit + integration (новые fixtures) |
| **Удаление** | -2800 | bridge/ полностью (2010 Go + 244 prompt + ~500 fixtures) |
| **Итого нового** | ~800 | + ~1500 тестов |

**Stories:** 4-6 (parser, progress tracker, runner refactor, execute.md, integration tests, CLI changes).
**Дни:** 3-5.

### 6. Качество декомпозиции: 8/10

**Сильные стороны:**
- **Полный контекст:** Claude видит ВСЮ story (AC, Dev Notes, Architecture references, Testing Standards) с первого раза. Нет потери информации "испорченного телефона"
- **2 AI-слоя вместо 3:** BMad (stories) -> Runner (код). Bridge устранён
- **Один source of truth:** story файл = единственная спецификация
- **Claude сам решает granularity:** Runner'ский Claude лучше bridge'евого понимает что делать, потому что видит и код, и требования

**Слабые стороны:**
- **Нет checkpoint'ов внутри story:** если story содержит 8 AC, Claude пытается реализовать всё за одну сессию. При неудаче — полный откат
- **Story может быть слишком большой:** story с 10+ AC и 200+ строк Dev Notes может не поместиться в контекст с кодом
- **Нет granular tracking:** нельзя сказать "AC-1 и AC-2 выполнены, AC-3 — нет" (только story-level: done/in-progress)

### 7. Скорость (time-to-first-task)

- BMad pipeline: 45-110 мин (PRD + Architecture + Epics + Stories + Validation)
- Bridge: **0 мин** (удалён)
- Runner: 5-15 мин
- **Итого: 50-125 мин** (bottleneck = BMad)

Экономия: 10-30 мин (bridge полностью устранён). Незначительное улучшение — BMad доминирует.

### 8. Стоимость

- Planning: **$0** (нет bridge, нет planner)
- BMad setup: $2-8 за проект
- Execution: потенциально дороже — Claude видит полную story (больше токенов в промпте)
- **Итого за проект: $2-9** (bridge экономия нивелируется большими промптами)

### 9. Гибкость: 1/10

- Работает ТОЛЬКО с BMad story файлами
- story_parser.go привязан к конкретной структуре
- Новые форматы = новые парсеры
- Хуже варианта A: runner теперь жёстко привязан к story формату (раньше runner не знал о stories — работал с sprint-tasks.md)

### 10. Backward compatibility: Нет

- `ralph bridge` удалён — существующие скрипты ломаются
- sprint-tasks.md больше не генерируется — workflow зависящие от него ломаются
- `runner.Execute()` API изменён — тесты переписываются
- Progress tracking формат новый — старый state несовместим
- **Критически:** scan.go полностью переписан — 20+ ссылок в тестах runner'а потенциально ломаются

---

## Вариант C: Stories убираются, ralph работает с epics

### Суть

Убрать промежуточный слой stories. Epic файл содержит все requirements, ralph обрабатывает epic целиком — от декомпозиции до выполнения.

### 1. Архитектура

```
cmd/ralph/
├── run.go             # --epic flag вместо stories/sprint-tasks
runner/
├── epic_parser.go     # ParseEpic() — извлечение stories/requirements из epic
├── decomposer.go      # DecomposeEpic() — разбиение epic на сессии
├── progress.go        # EpicProgressTracker
├── runner.go          # Execute() — epic-based workflow
├── prompts/
│   └── execute.md     # Epic context injection
bridge/
  # УДАЛЁН
```

**Пакеты:** `runner/` (значительная переработка).
**Промпты:** execute.md значительно переработан — epic-level context.
**Команды:** `ralph run --epic docs/epics/epic-10.md`.

### 2. Входные данные

- BMad epic файлы (`docs/epics/epic-N-*.md`)
- Содержат: список stories, FR-маппинг, scope, dependencies
- Опционально: architecture docs для технического контекста
- Без BMad epics — ralph неработоспособен

### 3. Выходные данные

- Код
- `.ralph-epic-state.yaml` — прогресс на уровне epic:
  ```yaml
  epic: epic-10-context-window-observability
  stories:
    10.1: done
    10.2: in-progress
    10.3: pending
  ```
- Нет sprint-tasks.md, нет story-level tracking

### 4. Зависимости

| Зависимость | Статус |
|-------------|--------|
| BMad Method | **Полная** — epics обязательны |
| LLM (planning) | **Частичная** — LLM нужен для декомпозиции epic на сессии |
| LLM (execution) | Значительно больше — epic context = сотни строк |
| Формат epic | **Жёсткая** — парсер привязан к структуре |
| Формат story | **Не используется** (stories внутри epic) |

### 5. Объём реализации

| Компонент | LOC | Описание |
|-----------|-----|----------|
| `epic_parser.go` | ~200 | Парсинг epic: stories, FR, scope |
| `decomposer.go` | ~300 | Декомпозиция epic на сессии (LLM или программно) |
| `progress.go` | ~250 | Epic+story level state management |
| Рефакторинг `runner.go` | ~250 | Epic-based workflow |
| Промпт переработка | ~100 | execute.md для epic context |
| Тесты | ~2000 | Полностью новые fixtures |
| **Удаление** | -2800 | bridge/ + scan.go (текущий) |
| **Итого нового** | ~1200 | + ~2000 тестов |

**Stories:** 6-8 (epic parser, decomposer, progress, runner refactor, prompts, CLI, integration, edge cases).
**Дни:** 5-7.

### 6. Качество декомпозиции: 4/10

**Сильные стороны:**
- **Полный контекст связей:** Claude видит все stories в epic и их зависимости
- **Один документ:** epic = единый source of truth для всего scope

**Слабые стороны:**
- **Context window overflow:** Epic файл bmad-ralph содержит ~200 строк. С architecture + codebase context = 500+ строк только metadata, не считая кода. Для epic из 10 stories context window переполняется
- **Потеря granularity:** нет story-level review points — человек видит только epic-level progress
- **Декомпозиция сложнее:** вместо "1 story = 1-3 tasks" нужно "1 epic = 10-30 tasks" — LLM декомпозиция на порядок сложнее
- **Epic — высокоуровневый документ:** содержит стратегические решения, scope определение, зависимости между stories. НЕ содержит implementation details (Dev Notes, Testing Standards). Эта информация теряется
- **Нет Dev Notes:** stories содержат детальные Dev Notes с архитектурными решениями, файловыми путями, edge cases. Epic не содержит этого уровня детализации

### 7. Скорость (time-to-first-task)

- BMad pipeline: 40-100 мин (PRD + Architecture + Epics, без stories)
- Decomposition: 3-10 мин (LLM декомпозиция epic)
- Runner: 5-15 мин
- **Итого: 48-125 мин**

Экономия: 10-20 мин (нет story creation/validation). Но BMad всё равно нужен для epics.

### 8. Стоимость

- Planning (decomposition): $0.30-1.00 (LLM декомпозиция epic)
- BMad setup: $1-4 (PRD + Architecture + Epics, без stories)
- Execution: дороже — epic context больше story context
- **Итого за проект: $1.30-5.00**

### 9. Гибкость: 1/10

- Работает ТОЛЬКО с BMad epic файлами
- Ещё более жёсткая привязка, чем к stories
- Нет альтернативного входа (Issues, plain text, PRD)

### 10. Backward compatibility: Нет

- sprint-tasks.md удалён
- `ralph bridge` удалён
- runner API полностью переработан
- Workflow кардинально другой: от "task-at-a-time" к "epic-at-a-time"
- Все существующие скрипты и интеграции ломаются

---

## Вариант D: ralph plan — самодостаточная команда

### Суть

Новая команда `ralph plan` принимает любой текстовый input (PRD, GitHub Issue, plain text, BMad stories) и генерирует sprint-tasks.md. BMad становится опциональным. Runner без изменений.

### 1. Архитектура

```
cmd/ralph/
├── plan.go            # НОВОЕ: CLI `ralph plan <file>`
├── bridge.go          # Deprecated (Phase 2), удалён (Phase 3)
planner/
├── planner.go         # Plan(ctx, cfg, inputFiles) — ядро
├── prompts/
│   └── plan.md        # Промпт-планировщик (~150 строк)
bridge/
├── ...                # Deprecated, затем удалён
runner/
├── scan.go            # БЕЗ ИЗМЕНЕНИЙ
├── runner.go          # БЕЗ ИЗМЕНЕНИЙ
```

**Пакеты:** `planner/` (новый), `cmd/ralph/plan.go` (новый). `bridge/` deprecated.
**Промпты:** `plan.md` (~150 строк) — значительно проще `bridge.md` (244 строки).
**Команды:** `ralph plan <file>` (новая), `ralph bridge` (deprecated).

**Dependency direction:**
```
cmd/ralph → planner → session, config     # НОВОЕ
cmd/ralph → runner  → session, config     # без изменений
cmd/ralph → bridge  → session, config     # deprecated
```

### 2. Входные данные

**Универсальный вход:**
- **PRD:** `ralph plan docs/prd/feature-x.md`
- **Plain text:** `echo "Добавь OAuth2" | ralph plan --stdin`
- **GitHub Issues:** `gh issue view 42 --json body -q .body | ralph plan --stdin`
- **BMad stories:** `ralph plan docs/sprint-artifacts/9-1-*.md` (stories = просто markdown)
- **Architecture context:** `ralph plan prd.md --architecture docs/architecture.md`

**Ключевое:** формат входных данных НЕ фиксирован. Любой markdown/text.

### 3. Выходные данные

- `sprint-tasks.md` — **тот же формат**, что генерирует bridge сейчас
- `- [ ]` / `- [x]` с `source:` полями
- `[GATE]` маркеры
- runner парсит без изменений

**Source traceability адаптирована:**
- Из PRD: `source: prd.md#FR-3`
- Из story: `source: 9-1-progressive-review.md#AC-1`
- Из stdin: `source: inline#1`

### 4. Зависимости

| Зависимость | Статус |
|-------------|--------|
| BMad Method | **Нулевая** — BMad опционален |
| LLM (planning) | **Частичная** — для сложных requirements (5+ FR) |
| LLM (execution) | Без изменений (runner) |
| Формат входа | **Гибкий** — любой markdown/text |
| sprint-tasks.md формат | Без изменений |

### 5. Объём реализации

| Компонент | LOC | Описание |
|-----------|-----|----------|
| `planner/planner.go` | ~80 | Plan() — аналог bridge.Run() |
| `planner/prompts/plan.md` | ~150 | Промпт-планировщик |
| `cmd/ralph/plan.go` | ~40 | CLI-команда |
| Тесты | ~1250 | Unit + prompt + integration |
| **Удаление (Phase 3)** | -2844 | bridge/ полностью |
| **Итого нового** | ~120 Go | + ~150 промпт + ~1250 тестов |

**Нетто-эффект (после Phase 3): -1324 строки** (упрощение codebase).

**Stories:** 3-5 (planner core + prompt, CLI + stdin, integration tests, bridge deprecation, bridge removal).
**Дни:** 2-3.

### 6. Качество декомпозиции: 7/10

**Сильные стороны:**
- **Codebase awareness:** Plan-промпт включает tree output проекта, go.mod, CLAUDE.md — bridge этого не имеет
- **2 AI-слоя вместо 3:** Plan (задачи) -> Runner (код). BMad и bridge устранены
- **FR прямо в задачу:** вместо "story -> AC -> task description (2 строки) -> source file", план включает FR-контекст непосредственно в task description
- **Нет batching:** PRD (10-50KB) помещается в один контекст. 6 batch'ей bridge'а устранены
- **ADaPT-совместимость:** для простых FR — программная генерация, для сложных — LLM

**Слабые стороны:**
- **LLM недетерминизм:** одни и те же requirements могут дать разные задачи (хотя и менее хрупко, чем bridge — промпт на 40% короче)
- **Зависимость от качества input:** "добавь auth" даст менее качественные задачи, чем формализованный PRD
- **Нет BMad-уровня валидации:** BMad validate-workflow проверяет полноту, testability AC. ralph plan этого не делает

### 7. Скорость (time-to-first-task)

**Без BMad (основной сценарий):**
- Написание requirements: 5-15 мин (человек)
- `ralph plan`: 1-3 мин (1-2 Claude calls)
- `ralph run`: 5-15 мин
- **Итого: 11-33 мин**

**С BMad stories:**
- BMad pipeline: 50-120 мин
- `ralph plan stories.md`: 1-3 мин
- `ralph run`: 5-15 мин
- **Итого: 56-138 мин** (BMad — bottleneck)

**Конкурентное сравнение (без BMad):**

| Инструмент | Time-to-first-task |
|-----------|-------------------|
| Aider | 1-2 мин |
| Claude Code | 1-3 мин |
| Devin | 2-5 мин |
| SWE-Agent | 3-10 мин |
| **ralph plan (D)** | **11-33 мин** |
| ralph + BMad (текущий) | 55-130 мин |

ralph plan в **3-5x быстрее** текущего ralph, хотя всё ещё медленнее чисто интерактивных инструментов (что ожидаемо для batch-executor).

### 8. Стоимость

- Planning: **$0.10-0.30** (1-2 Claude calls для декомпозиции)
- BMad setup: **$0** (не нужен)
- Execution: без изменений
- **Итого за проект: $0.10-0.30** (planning) — экономия 90-97% по сравнению с текущим ($2.50-9.80)

### 9. Гибкость: 9/10

- **PRD:** `ralph plan prd.md`
- **GitHub Issues:** `gh issue view 42 | ralph plan --stdin`
- **Plain text:** `echo "add OAuth2" | ralph plan --stdin`
- **BMad stories:** `ralph plan story1.md story2.md`
- **YAML tasks:** можно добавить позже как формат ввода
- **Jira/Linear:** через CLI-export + `--stdin`

Единственное ограничение: выход всегда sprint-tasks.md (runner constraint). Но это feature, не bug — sprint-tasks.md = отлаженный формат с 42 stories опыта.

### 10. Backward compatibility: Полная

- **runner без изменений:** scan.go парсит sprint-tasks.md как раньше
- **sprint-tasks.md формат:** идентичен
- **`ralph bridge`:** deprecated (Phase 2) с warning, удалён (Phase 3)
- **Существующие stories:** работают как input для `ralph plan`
- **Миграция для пользователей:** `ralph bridge story.md` -> `ralph plan story.md` (1 слово)
- **0 тестов runner'а ломается** (runner не знает откуда sprint-tasks.md)

---

## Развёрнутый анализ: ключевые dimensions

### I. Устранение проблемы "испорченного телефона"

| Вариант | AI-слоёв | Потеря информации | Комментарий |
|---------|---------|-------------------|-------------|
| A | 2 (BMad + Runner) | Средняя (AC -> task программно) | Bridge не теряет, но BMad теряет при story creation |
| B | 2 (BMad + Runner) | **Низкая** (полная story в промпте) | Лучший результат — Claude видит всё |
| C | 2 (BMad + Runner) | Высокая (epic = высокоуровневый) | Dev Notes теряются (они в stories, а stories удалены) |
| D | 2 (Plan + Runner) | Низкая (FR прямо в задачу) | Нет промежуточного слоя stories |

**Победитель:** B (минимальная потеря) или D (устраняет BMad-слой целиком).

### II. Самодостаточность

| Вариант | Работает без BMad? | Работает без LLM (planning)? | Онбординг |
|---------|-------------------|------------------------------|-----------|
| A | Нет | Да | Сложный (установить BMad) |
| B | Нет | Да | Сложный (установить BMad) |
| C | Нет | Нет (нужен decomposer) | Сложный (установить BMad) |
| D | **Да** | Частично (простые FR — да) | **Простой** (`ralph plan file.md`) |

**Победитель:** D (единственный самодостаточный).

### III. Риск реализации

| Вариант | Риск | Причины |
|---------|------|---------|
| A | **Низкий** | Минимальные изменения, runner не трогается |
| B | **Средний** | runner переписывается, новый progress tracker, новые промпты |
| C | **Высокий** | Полный редизайн workflow, потеря Dev Notes, context overflow |
| D | **Низкий** | Новый пакет параллельно, runner не трогается, bridge deprecated |

**Победитель:** A и D (минимальный риск).

### IV. Масштабируемость

| Вариант | Малый проект (1-5 FR) | Средний проект (5-20 FR) | Большой проект (20+ FR) |
|---------|----------------------|-------------------------|------------------------|
| A | Работает (если есть stories) | Работает | Работает (программный, нет batching) |
| B | Работает (если есть stories) | Работает | Проблема: большие stories → context overflow |
| C | Overkill | Работает | **Не работает**: epic + code = context overflow |
| D | `echo "fix bug" \| ralph plan --stdin` | `ralph plan prd.md` | `ralph plan` с итеративной стратегией (фаза 1: группировка, фаза 2: декомпозиция по эпикам) |

**Победитель:** D (адаптируется к масштабу).

### V. Влияние на community adoption

| Вариант | Новые пользователи | Существующие пользователи BMad | Open-source appeal |
|---------|-------------------|-----------------------------|-------------------|
| A | Барьер: нужен BMad | Без изменений | Низкий (niche tool) |
| B | Барьер: нужен BMad | Переучиваться (новый workflow) | Низкий |
| C | Барьер: нужен BMad | Переучиваться (новый workflow) | Низкий |
| D | **Нет барьера** | Плавная миграция | **Высокий** (self-contained CLI) |

**Победитель:** D (максимальный потенциал adoption).

---

## Матрица решения (взвешенные оценки)

| Критерий | Вес | A | B | C | D |
|----------|-----|---|---|---|---|
| Качество декомпозиции | 20% | 6 | **8** | 4 | 7 |
| Самодостаточность | 15% | 1 | 1 | 1 | **10** |
| Time-to-first-task | 15% | 3 | 3 | 3 | **8** |
| Объём работы (обратный) | 10% | **9** | 5 | 3 | 8 |
| Backward compatibility | 10% | **10** | 2 | 1 | **10** |
| Гибкость форматов | 10% | 1 | 1 | 1 | **9** |
| Риск реализации (обратный) | 10% | **9** | 6 | 3 | **9** |
| Стоимость (обратный) | 5% | 7 | 6 | 7 | **9** |
| Adoption потенциал | 5% | 2 | 2 | 2 | **9** |
| **Итого (взвешенный)** | | **4.85** | **3.95** | **2.45** | **8.50** |

---

## Итоговая рекомендация

### Рекомендуемый вариант: D (ralph plan)

**Обоснование по 5 ключевым аргументам:**

**1. Конкурентный паритет.** Из 9 рассмотренных инструментов (Devin, Claude Code, Aider, SWE-Agent, OpenHands, AutoCodeRover, Cursor, Windsurf, MetaGPT) **ни один** не требует внешнего workflow для создания входных данных. ralph с BMad-зависимостью — аномалия рынка. Вариант D устраняет эту аномалию.

**2. Минимальный риск при максимальном эффекте.** planner/ — новый пакет параллельно bridge/. Runner не трогается. 0 существующих тестов ломается. Bridge deprecated, не удалён немедленно. При этом эффект максимальный: самодостаточность, 90% экономия на planning, 3-5x ускорение time-to-first-task.

**3. ADaPT-совместимость.** Исследование Allen AI (NAACL 2024) показывает: as-needed decomposition на 28-33% эффективнее upfront decomposition. Bridge делает upfront (все AC -> все tasks). Plan может использовать ADaPT: простые FR -> программная генерация, сложные FR -> LLM декомпозиция.

**4. Нетто-упрощение.** После Phase 3 (удаление bridge): -1324 строки кода. Codebase становится проще, не сложнее.

**5. Backward compatibility.** Единственный вариант (кроме A), который полностью backward-compatible. Существующие sprint-tasks.md работают. BMad stories работают как input для `ralph plan`. Миграция = замена одного слова в команде.

### Почему НЕ другие варианты

| Вариант | Причина отклонения |
|---------|-------------------|
| **A** | Решает тактическую проблему (LLM в bridge), но не решает стратегическую (BMad-зависимость). ralph остаётся niche tool для пользователей BMad |
| **B** | Хорошее качество декомпозиции, но ломает backward compatibility, не устраняет BMad-зависимость, и runner становится более сложным |
| **C** | Худший вариант: context overflow для больших epics, потеря Dev Notes, полный редизайн, максимальный объём работы |

### Возможная комбинация: D + элементы A

Если качество программного парсинга AC (вариант A) окажется достаточным для BMad stories, можно комбинировать:
- **D** для новых проектов (plain text / PRD / Issues)
- **A-элементы** внутри D: когда input = BMad story, программный парсинг AC без LLM (0 cost)
- Это ADaPT-подход: программный парсинг для structured input, LLM для unstructured

### План реализации

| Фаза | Что | Усилия | Backward compatible |
|------|-----|--------|---------------------|
| Phase 1 | `ralph plan` — новая команда | 2-3 stories, 1-2 дня | Да (bridge работает) |
| Phase 2 | `ralph bridge` deprecated | 1 story, 0.5 дня | Да (warning) |
| Phase 3 | bridge/ удалён | 1 story, 0.5 дня | Нет |
| **Итого** | | **4-5 stories, 2-3 дня** | — |

---

## Источники

### Исследования
- [ADaPT: As-Needed Decomposition and Planning (NAACL 2024)](https://arxiv.org/abs/2311.05772)
- [Requirements are All You Need (2024)](https://arxiv.org/pdf/2406.10101)
- [The Agentic Telephone Game](https://www.christopheryee.org/blog/agentic-telephone-game-cautionary-tale/)
- [Why Do Multi-Agent LLM Systems Fail?](https://arxiv.org/pdf/2503.13657)

### Инструменты
- [Devin 2.0 Interactive Planning](https://docs.devin.ai/work-with-devin/interactive-planning)
- [Claude Code Best Practices](https://www.anthropic.com/engineering/claude-code-best-practices)
- [CCPM — Claude Code Project Manager](https://github.com/automazeio/ccpm)
- [claude-code-skills (levnikolaevich)](https://github.com/levnikolaevich/claude-code-skills)

### Внутренние исследования проекта
- `docs/research/bridge-concept-analysis.md`
- `docs/research/variant-d-self-sufficient-decomposition.md`
- `docs/research/variant-d-cost-benefit.md`
- `docs/research/variant-d-migration-path.md`
