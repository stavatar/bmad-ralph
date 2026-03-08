---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]
inputDocuments:
  - docs/product-brief-bmad-ralph-2026-02-24.md
  - docs/brainstorming-session-2026-02-23.md
  - docs/research/new_version/SUMMARY.md
workflowType: 'prd'
lastStep: 11
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-03-07'
---

# Product Requirements Document — bmad-ralph v2

**Author:** Степан
**Date:** 2026-03-07

---

## Executive Summary

ralph v2 — самодостаточный CLI-оркестратор Claude Code для автономной разработки.
Центральное изменение v2: новая команда `ralph plan` читает входные документы
и генерирует `sprint-tasks.md` напрямую, без BMad bridge.

**Два режима работы:**
- **BMad-режим**: автодискавери `docs/` (prd.md, architecture.md, front-end-spec.md и др.)
- **Single-doc режим**: один файл с требованиями (`ralph plan requirements.md`)
- Переключение через `config.yaml` (`plan_mode: bmad|single`) или автодетект по количеству файлов

**Ключевые сценарии MVP:**
- **Merge mode**: добавление задач к существующему `sprint-tasks.md` без пересоздания с нуля
- **Стек-нейтральность**: Go, TypeScript, Python — `plan.md` не привязан к языку
- **Большие документы**: при превышении ~50K токенов ralph plan выводит предупреждение с рекомендацией разбить документ (например через `bmad shard-doc`) и завершает с ненулевым exit code. Обработка больших документов вне скоупа MVP.

**Primary user (MVP):** разработчики, уже использующие BMad Method — получают немедленное улучшение: замена bridge на более быстрый и точный ralph plan.
**Secondary user (v2.1):** разработчики без BMad (через `ralph init`).

### Что делает ralph особенным

Единственный CLI с замкнутым циклом обучения:
**planning → execution → review → knowledge extraction → distillation → injection.**
Каждый запуск ralph накапливает знания, которые улучшают следующий.
Конкуренты (Devin, Kiro, Cursor, Aider) stateless — ralph обучается.

## Project Classification

**Technical Type:** CLI Tool + Developer Tool
**Domain:** AI-assisted software development / DevTools
**Complexity:** Medium-High
**Primary Segment (MVP):** BMad Method users
**Secondary Segment (v2.1):** Разработчики без BMad (через ralph init)

---

## Success Criteria

### User Success

**Главный момент "ага, работает!":**
Пользователь запускает `ralph plan` и получает `sprint-tasks.md` который можно
передать в `ralph run` без единого ручного исправления задач.

**AI Plan Review — встроен в основной flow:**
Сразу после генерации `sprint-tasks.md` ralph автоматически запускает
reviewer в чистой сессии Claude. Управляется флагом `--no-review`.

```
ralph plan docs/              # plan + auto review (default)
ralph plan docs/ --no-review  # только plan, без review
```

Flow с прогресс-индикатором:
```
✓ Генерация плана...
✓ AI review плана...          # пропускается с --no-review
→ OK: sprint-tasks.md готов
→ Issues найдены: максимум 1 автоматический retry
  Если после retry проблемы остаются:
  ⚠ Report + gate: [p]roceed / [e]dit / [q]uit
```

Reviewer оценивает план по критериям:
- Покрытие всех FR из входных документов
- Гранулярность задач (не слишком крупные, не слишком мелкие)
- Корректность source-ссылок
- Наличие SETUP + GATE задач
- Отсутствие дублирования и противоречий

**Открытый вопрос (требует research story):**
Как автоматически измерить качество плана на learnPracticsCodePlatform?
Нужны объективные метрики помимо ручного review.

### Business Success

**Минимум — известный open source продукт:**
- Публичный репозиторий с README, документацией, примерами
- Внешние пользователи используют ralph на реальных проектах
- Упоминания в developer communities (HN, Reddit r/golang, X/Twitter)

**Максимум — монетизация:**
- ralph для команд: shared knowledge base, team config sync
- Потенциально: веб-интерфейс как отдельный продукт (v3+)

### Technical Success

**Acceptance Test:** learnPracticsCodePlatform
- `ralph plan docs/` генерирует `sprint-tasks.md` из реальных BMad-документов
- Сравнение с `sprint-tasks.old.md` (предыдущая версия bridge, 295 строк)
- Метрика качества: TBD (отдельный research story)

**Тестируемость auto review:**
Два обязательных mock-сценария в тестах:
- Review возвращает OK → план принят
- Review возвращает issues → retry → gate

### Measurable Outcomes (MVP)

| Метрика | Сейчас (bridge) | Цель (ralph plan) |
|---------|----------------|-------------------|
| Time-to-first-task | 55-130 мин | ≤33 мин |
| Стоимость (--no-review) | $2.50-9.80 | ≤$0.30 |
| Стоимость (default + review) | $2.50-9.80 | ≤$0.60 |
| Битых source-ссылок | 100% | 0% |
| Сломанных runner тестов | — | 0 |
| Нетто-эффект кода | — | ≥-1000 строк |

---

## Product Scope

### MVP (Epic 11 + Epic 15) — 2.5 дня

- `ralph plan [файлы...]` — основная команда
- **BMad-режим**: автодискавери `docs/` по настраиваемому списку в `config.yaml`
  (`plan_input_files`). Дефолт: `prd.md`, `architecture.md`, `front-end-spec.md`, `ux-design.md`
- **Single-doc режим**: `ralph plan requirements.md`
- **Merge mode**: добавление задач к существующему `sprint-tasks.md`
- **Auto review**: встроенный AI-reviewer в чистой сессии, флаг `--no-review` для отключения
- Предупреждение при документах >50K токенов (рекомендация `bmad shard-doc`)
- `ralph bridge` deprecated с warning → удалён (-2844 строк кода)
- Acceptance test: learnPracticsCodePlatform

### Growth Features (Epic 12 + Epic 14) — +1.5 дня

- `ralph replan` — коррекция курса через gate `[c]` без перезапуска
- `ralph init "описание"` — быстрый старт без BMad за 2-3 мин
- Research story: объективные метрики качества плана

### Vision (Future)

- ralph для команд: shared knowledge base, team config sync
- YAML формат задач (Epic 13) — после валидации на практике
- Веб-интерфейс как отдельный продукт (v3+)
- Монетизация premium фич

---

## User Journeys

### Journey 1: Артём — BMad user, первый запуск без bridge

Артём открывает терминал в пятницу вечером. Перед ним 6 готовых BMad stories и привычная боль: раньше он запускал `ralph bridge`, ждал 25 минут, потом ещё полчаса вручную правил задачи с битыми source-ссылками.

На этот раз набирает `ralph plan docs/`. Через 50 секунд:
```
✓ Генерация плана...  (32 сек)
✓ AI review плана...  (18 сек)
→ OK: 47 задач сгенерировано, 0 issues
```
Source-ссылки корректные. Dev Notes из stories попали в задачи. Запускает `ralph run` — не исправив ни одной строки. Первый раз за три месяца.

*Выявленные требования: BMad autodiscovery, role mapping по именам файлов, source refs, Dev Notes extraction, auto review.*

---

### Journey 2: Артём — середина спринта, добавляет фичу

Вторник, половина задач выполнена ([x]). Клиент просит добавить экспорт в CSV. Артём дополняет `prd.md`, запускает `ralph plan docs/ --merge`. Ralph читает существующий `sprint-tasks.md`, видит выполненные задачи, добавляет только новые задачи для CSV-экспорта — не трогая уже сделанное.

*Выявленные требования: merge mode с защитой [x] задач, инкрементальное планирование.*

---

### Journey 3: Лена — без BMad, single-doc режим

Лена пишет `requirements.md` — 2 страницы описания задачи. Запускает `ralph plan requirements.md`. Ralph предупреждает: "документ 60K токенов — рекомендую разбить через `bmad shard-doc`". Лена разбивает на три файла, запускает снова. 40 секунд — 23 задачи, без ошибок. Все настройки ролей и BMad-маппинга игнорируются — файл читается как единый контекст.

*Выявленные требования: single-doc режим, size warning с рекомендацией shard-doc, role mapping отключён в single-doc.*

---

### Journey 4: Степан — настройка под нестандартные имена файлов

Новый проект с нестандартными именами: `spec.md`, `technical-design.md`. Степан открывает `ralph.yaml`:
```yaml
plan_input_files:
  - file: spec.md
    role: requirements
  - file: technical-design.md
    role: technical_context
```
`ralph plan` автодискавери находит эти файлы, передаёт LLM с правильными ролями в заголовках:
`<!-- file: spec.md | role: requirements -->`.

**Логика ролей по умолчанию (BMad-режим):**

| Имя файла | Роль автоматически |
|-----------|-------------------|
| `prd.md` | `requirements` — источник FR |
| `architecture.md` | `technical_context` — стек, ограничения |
| `ux-design.md` | `design_context` — UI детали |
| `front-end-spec.md` | `ui_spec` — компоненты |

Явный маппинг в config переопределяет дефолты. В single-doc режиме все роли игнорируются.

*Выявленные требования: настраиваемый `plan_input_files` с опциональными ролями, дефолтный маппинг по именам файлов BMad.*

---

### Journey Requirements Summary

| Journey | Ключевые требования |
|---------|-------------------|
| Артём — первый запуск | BMad autodiscovery, role mapping, source refs, Dev Notes, auto review |
| Артём — merge | Merge mode, защита [x] задач, инкрементальное планирование |
| Лена — single doc | Single-doc режим, size warning, роли отключены |
| Степан — конфиг | `plan_input_files` с ролями, дефолтный BMad-маппинг |

---

## Innovation & Novel Patterns

### Detected Innovation Areas

**1. Замкнутый цикл накопления знаний (Run Mode)**

В режиме `ralph run` реализован полный цикл:
`planning → execution → review → knowledge extraction → distillation → injection`

Каждый запущенный sprint накапливает знания в knowledge base, которые автоматически инжектируются в следующий цикл выполнения. Конкуренты (Devin, Kiro, Cursor, Aider) stateless — у ralph каждый следующий *sprint* выполняется лучше предыдущего без дополнительных усилий разработчика.

**2. Typed Inputs for LLM (Architectural Pattern)**

Входные документы передаются LLM с явными семантическими типами через HTML-комментарии:
`<!-- file: prd.md | role: requirements -->`

LLM получает не просто текст, а типизированный контекст: `requirements` → FR-coverage check, `technical_context` → feasibility check, `design_context` → UI fidelity check. Это архитектурный паттерн, обеспечивающий качество вывода без увеличения размера промпта.

**3. AI-assisted Planning с встроенным Review**

Auto review работает в чистой Claude-сессии сразу после генерации плана. Reviewer оценивает объективно (без bias основной сессии), фиксируя issues до передачи плана в выполнение. Это одноразовый quality gate, не накопительное обучение.

### Market Context & Competitive Landscape

| Инструмент | Stateless run? | Learning loop? | Typed doc inputs? | Built-in review? |
|------------|---------------|---------------|-------------------|-----------------|
| Devin | ✅ да | ❌ | ❌ | ❌ |
| Cursor | ✅ да | ❌ | ❌ | ❌ |
| Aider | ✅ да | ❌ | ❌ | ❌ |
| Kiro | ✅ да | ❌ | Частично | Частично |
| **ralph v2** | ❌ **stateful** | ✅ **run mode** | ✅ **typed roles** | ✅ **plan mode** |

**Evangelist scenario:** Артём — опытный разработчик, которому Aider и Cursor уже недостаточны. Он хочет систему, которая *помнит* что работало раньше и генерирует план без ручных правок. Через 3 месяца пишет пост на HN: "ralph — первый инструмент, где я не исправляю ни одной задачи после plan".

### Validation Approach

- **Acceptance test**: learnPracticsCodePlatform — реальный проект, сравнение с предыдущим bridge output
- **Typed inputs validation**: количество битых source-ссылок = 0% (было 100% с bridge)
- **Learning loop**: качество sprint execution улучшается после N итераций (research story)
- **Open Standard**: публикация role-mapping specification → community ecosystem, привлечение внешних пользователей

### Risk Mitigation

- **Риск: typed inputs хрупкие** → Дефолты для BMad имён, ручная настройка через `plan_input_files` в config
- **Риск: auto review замедляет flow** → Флаг `--no-review`, benchmark цель ≤60 сек overhead
- **Риск: learning loop не улучшает quality** → Fallback: knowledge base как проектная документация
- **Риск: stateful knowledge устаревает** → `ralph distill` перезаписывает, не накапливает мусор

---

## CLI Tool Specific Requirements

### Project-Type Overview

ralph v2 — CLI-первый инструмент для разработчиков, работающий в терминале.
Ключевые характеристики: предсказуемость поведения, правильные exit codes,
scriptability, минимальные зависимости при установке.

### Command Architecture

| Команда | Статус | Описание |
|---------|--------|----------|
| `ralph plan [файлы...]` | **MVP** | Генерация sprint-tasks.md из входных документов |
| `ralph plan --merge` | **MVP** | Добавление задач к существующему файлу |
| `ralph plan --no-review` | **MVP** | Без AI review |
| `ralph plan --force` | **MVP** | Игнорировать size warning (для CI/CD) |
| `ralph plan --output <path>` | **MVP** | Альтернативный путь output файла (дефолт: sprint-tasks.md) |
| `ralph run` | Existing | Выполнение sprint-tasks.md |
| `ralph bridge` | Deprecated | Удаляется в MVP (→ -2844 строк) |
| `ralph replan` | Growth | Коррекция курса через gate |
| `ralph init "описание"` | Growth | Быстрый старт без BMad |

### Flag Combinations

| Комбинация | Поведение |
|------------|-----------|
| `--merge` (без файла) | Создать новый файл (не ошибка) |
| `--merge --no-review` | Merge без AI review |
| `--force --no-review` | Пропустить size check и review |
| `--output path --merge` | Merge в указанный файл |

### Configuration

```yaml
# ralph.yaml — конфигурация в корне проекта
plan_mode: bmad          # bmad | single | auto (автодетект)
plan_input_files:        # опционально — переопределяет дефолты
  - file: prd.md
    role: requirements
  - file: architecture.md
    role: technical_context
```

Дефолтный BMad-маппинг (без явного конфига):

| Файл | Роль |
|------|------|
| `prd.md` | `requirements` |
| `architecture.md` | `technical_context` |
| `ux-design.md` | `design_context` |
| `front-end-spec.md` | `ui_spec` |

### Environment Integration

- **Shell**: работает в bash/zsh/fish без дополнительных зависимостей
- **CI/CD**: scriptable — exit code 0 = success, non-zero = failure; `--force` для pipeline
- **Claude Code**: использует `config.ClaudeCommand` для запуска Claude сессий
- **Предупреждение >50K токенов**: exit 2 + stderr; с `--force` продолжает с exit 0

### Exit Codes

| Код | Значение |
|-----|----------|
| 0 | Успех |
| 1 | Общая ошибка |
| 2 | Документы >50K токенов (без `--force`) |
| 3 | Review gate: пользователь выбрал quit |

### Architecture Constraint

Пакет `plan` принимает `[]PlanInput{File, Role}` — typed structs, не strings.
Это позволяет `ralph replan` переиспользовать логику без рефакторинга.

### Distribution

- **Go binary**: single binary, CGO_ENABLED=0, linux/darwin/windows
- **goreleaser**: автоматическая сборка через GitHub Actions
- **Установка**: `go install` или binary с GitHub Releases
- **Без внешних зависимостей в runtime** (только Claude CLI)

---

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-Solving MVP

**Цель (Jobs-to-be-Done):** Первый инструмент, где разработчик запускает `ralph plan`
и сразу переходит к `ralph run` — без единого ручного исправления.

**Исполнитель:** 1 разработчик (Степан), solo dev workflow.

### MVP Feature Set — Epic 11 + Epic 15 (~2.5 дня)

**Epic dependency:** Epic 15 (bridge removal) блокируется Epic 11 (ralph plan).
Bridge удаляется только после того, как план полностью работает.

**Поддерживаемые journeys:** Артём (первый запуск), Артём (merge), Лена (single doc), Степан (конфиг)

**Must-Have:**
- `ralph plan [файлы...]` — BMad-режим и single-doc режим
- Автодискавери `docs/` с настраиваемым `plan_input_files` в config
- Role mapping по именам файлов (typed inputs for LLM)
- Auto review в чистой Claude-сессии (флаг `--no-review`)
- Merge mode (`--merge`), защита выполненных [x] задач
- Size warning >50K токенов (exit 2, флаг `--force` для CI)
- `ralph bridge` deprecated с warning → удалён (-2844 строк)
- Acceptance test: learnPracticsCodePlatform

### Growth Features — Epic 12 + Epic 14 (~+1.5 дня)

**Условие входа в Growth:** BMad-режим подтверждён на 3+ реальных проектах.

**Growth 0 (Research — блокирует остальные Growth):**
- Research story: объективные метрики качества плана — без этого неизвестно, стоит ли делать replan и init

**Growth 1:**
- `ralph replan` — коррекция курса через gate [c] без перезапуска

**Growth 2 (только после Growth 0 + условие входа):**
- `ralph init "описание"` — быстрый старт без BMad за 2-3 мин

### Vision — Future (v3+)

- YAML формат задач (Epic 13) — после валидации на практике
- ralph для команд: shared knowledge base, team config sync
- role-mapping spec как Open Standard → community ecosystem
- Веб-интерфейс как отдельный продукт
- Монетизация premium фич

### Risk Mitigation Strategy

**Технические риски:**
- *Typed inputs fragility* → дефолты для BMad, ручная настройка как fallback
- *LLM quality variability* → built-in auto review как safety net
- *Large docs performance* → size warning + `--force` escape hatch

**Рыночные риски:**
- *Adoption* → open source + acceptance test = живое доказательство ценности
- *ralph init подрывает репутацию* → запуск только после 3+ BMad проектов

**Ресурсные риски:**
- *Solo dev* → строгий MVP scope, bridge removal экономит 2844 строки
- *Если scope растёт* → Growth фичи откладываются, MVP не двигается

---

## Functional Requirements

### Планирование (ralph plan)

- FR1: Разработчик может запустить `ralph plan` в корне BMad-проекта и получить `sprint-tasks.md` из автоматически найденных документов в `docs/`
- FR2: Разработчик может передать явный список файлов: `ralph plan file1.md file2.md`
- FR3: Разработчик может запустить `ralph plan requirements.md` (single-doc режим) без BMad-структуры
- FR4: Разработчик может добавить новые задачи к существующему `sprint-tasks.md` через флаг `--merge`, не затрагивая выполненные задачи [x]
- FR5: В merge mode система пропускает stories, для которых уже есть задачи в существующем файле
- FR6: Разработчик может отключить AI review через флаг `--no-review`
- FR7: Разработчик может указать альтернативный путь output файла через `--output <path>`
- FR8: Разработчик может продолжить выполнение при документах >50K токенов через флаг `--force`

### Конфигурация и Role Mapping

- FR9: Разработчик может настроить список входных файлов с ролями через `plan_input_files` в `ralph.yaml`
- FR10: Система автоматически назначает роли файлам по их именам (prd.md→requirements, architecture.md→technical_context, ux-design.md→design_context, front-end-spec.md→ui_spec)
- FR11: Явный маппинг в `plan_input_files` переопределяет дефолтный маппинг по именам
- FR12: В single-doc режиме все роли игнорируются — файл читается как единый контекст
- FR13: Разработчик может переключить режим планирования (bmad/single/auto) через `plan_mode` в конфиге

### AI Review

- FR14: После генерации плана система автоматически запускает reviewer в чистой Claude-сессии
- FR15: Reviewer проверяет покрытие FR из входных документов, гранулярность задач, корректность source-ссылок, наличие SETUP/GATE задач, отсутствие дублирования
- FR16: При обнаружении issues система выполняет максимум 1 автоматический retry
- FR17: Если после retry проблемы остаются, система показывает report и gate: [p]roceed / [e]dit / [q]uit
- FR18: Разработчик может наблюдать прогресс генерации и review через progress-индикатор в реальном времени
- FR19: После завершения `ralph plan` система показывает summary: количество задач, статус review, путь output файла

### Качество плана

- FR20: Система передаёт входные документы LLM с typed headers `<!-- file: <name> | role: <role> -->`, обеспечивая точные source-ссылки с реальными именами файлов
- FR21: Система предупреждает разработчика при входных документах >50K токенов с рекомендацией `bmad shard-doc` и завершает с exit code 2 (без `--force`)
- FR22: При ошибке Claude API во время генерации система показывает actionable сообщение об ошибке и завершает с exit code 1

### Deprecation Bridge

- FR23: При вызове `ralph bridge` система показывает deprecation warning с указанием миграционного пути на `ralph plan`

### Коррекция курса (Run Mode)

- FR24: Gate задачи в `ralph run` позволяют разработчику давать feedback и корректировать выполнение текущего спринта без перезапуска

### Замкнутый цикл (Run Mode — существующий)

- FR25: Система накапливает знания из каждого выполненного спринта в knowledge base
- FR26: Система автоматически инжектирует накопленные знания в следующий цикл выполнения
- FR27: Разработчик может запустить `ralph distill` для перезаписи устаревших знаний

### Growth: Коррекция курса

- FR28: (Growth) Разработчик может запустить `ralph replan` для кардинального пересчёта плана с заменой незавершённых задач при сохранении выполненных [x]

### Growth: Быстрый старт

- FR29: (Growth) Разработчик может запустить `ralph init "описание"` для генерации минимального набора документов без BMad за 2-3 минуты

---

## Non-Functional Requirements

### Performance

- NFR1: `ralph plan --no-review` завершается за ≤20 минут на типичном BMad-проекте (4 документа, ~20K токенов)
- NFR2: `ralph plan` (с review) завершается за ≤33 минуты
- NFR3: Стоимость одного запуска `ralph plan --no-review` ≤ $0.30; с review ≤ $0.60
- NFR4: Progress-индикатор отображает каждую фазу (генерация / review / retry) до её завершения

### Security

- NFR5: Claude API ключ читается из переменной окружения или конфига — никогда не логируется и не попадает в output файлы
- NFR6: Входные документы не передаются третьим сторонам кроме Claude API

### Integration

- NFR7: ralph работает в bash/zsh/fish без дополнительных зависимостей в runtime
- NFR8: Exit codes (0/1/2/3) корректно проксируются в shell — ralph scriptable в CI/CD pipelines
- NFR9: `config.ClaudeCommand` позволяет подменить Claude бинарь для тестирования (mock support)
- NFR10: Совместим с BMad ecosystem: читает стандартные BMad-имена файлов без дополнительной конфигурации
- NFR11: ralph поставляется как single static binary без runtime зависимостей (CGO_ENABLED=0)

### Reliability

- NFR12: При любом прерывании или ошибке существующий `sprint-tasks.md` остаётся нетронутым
- NFR13: Повторный запуск `ralph plan` с теми же входными данными не повреждает существующий output файл
- NFR14: Merge mode не модифицирует существующие задачи — только добавляет новые в конец файла
- NFR15: При ошибке Claude API ralph возвращает actionable сообщение (не panic) с exit code 1

### Usability

- NFR16: Все error messages содержат actionable hint — конкретное действие для исправления ситуации
- NFR17: Deprecation warning для `ralph bridge` содержит точную команду-замену для copy-paste

### Maintainability

- NFR18: Покрытие тестами пакетов `plan` и `cmd/ralph` ≥ 80% (стандарт проекта)
- NFR19: Код `ralph bridge` удаляется полностью (0 мёртвого кода) до релиза MVP
