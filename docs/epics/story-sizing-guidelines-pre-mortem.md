# Story Sizing Guidelines (Pre-mortem)

Правила декомпозиции stories, выведенные из pre-mortem анализа. Каждая story должна быть выполнима **одним dev agent за одну сессию** (~25-35 turns).

### Target Story Size

| Параметр | Target | Max |
|----------|--------|-----|
| Production файлов | 1-2 | 3 |
| Acceptance criteria | 3-5 | 7 |
| Dev agent turns | ~25 | 35 |

Тесты пишутся **внутри** story, не отдельной story.

### Анти-паттерны декомпозиции

| Анти-паттерн | Правило |
|-------------|---------|
| "Весь package = одна story" | Max 1-3 файла, 3-7 AC |
| "Промпт + логика вместе" | Промпт-файлы = отдельная story от бизнес-логики |
| "Happy path + все edge cases" | Happy path first, edge cases = follow-up story |
| "Test infra + business logic" | Mock/testutil инфраструктура = foundation story |
| "N sub-components = 1 story" | Группировка по 3-5 элементов max |

### Structural Rules (Pre-mortem #2)

| # | Правило | Обоснование |
|---|---------|-------------|
| 1 | **Walking skeleton** в Epic 1 — минимальный e2e pass (config → session → execute one task) | Ранняя валидация integration path |
| 2 | **Project scaffold** story — directory structure + placeholder файлы из Architecture | Устраняет naming/path mismatches между stories |
| 3 | **Test infrastructure** story — mock Claude + mock git ДО бизнес-логики | Разблокирует все тестовые stories |
| 4 | **Промпты = first-class stories** с golden file AC, ДО logic stories | Промпты готовы когда логика их embed-ит |
| 5 | **Cross-cutting foundation** — error types, ctx pattern, exit code types | Единообразие error handling с первого дня |
| 6 | **Integration story** в конце каждого эпика | End-to-end валидация, не только unit |
| 7 | **Explicit prerequisites** — "Requires: Story X.Y", не "previous story" | Чёткий dependency graph |
| 8 | **Shared contracts** (sprint-tasks format) = отдельная story с tests в обоих packages | Предотвращает format divergence между bridge и runner |

### Анти-паттерны декомпозиции (дополнение)

| Анти-паттерн | Правило |
|-------------|---------|
| "Промпты = afterthought" | Промпт stories = first-class, с AC на golden file, ДО logic |
| "Dependencies implicit" | Каждая story: explicit "Requires: Story N.M" |
| "Cross-cutting ничейные" | Foundation story для error types, ctx, exit codes |
| "Всё с mock, нет integration" | Integration story в конце каждого эпика |
| "Scaffold по ходу дела" | Project scaffold = одна из первых stories |
| "Golden file = достаточно для промптов" | Scenario-based integration tests для КАЖДОГО типа промпта: mock Claude output → проверка контракта промпт↔парсер |

### Risk Heatmap по FR-кластерам (Failure Mode Analysis)

| Кластер | Risk | Ключевая угроза |
|---------|------|-----------------|
| **Review (FR13-FR18a)** | HIGHEST | False positive findings → бесконечный fix-цикл; state inconsistency при `[x]` + clear findings |
| **Execute (FR6-FR12)** | HIGH | Каскад: bad JSON → no session_id → no resume → lost progress |
| **Knowledge (FR26-FR29)** | HIGH | Перезапись CLAUDE.md вне ralph-секции; distillation убивает ценные паттерны |
| **Bridge (FR1-FR5a)** | MEDIUM | Smart Merge (FR4) может сбросить `[x]` → потеря прогресса |
| **Gates (FR20-FR25)** | MEDIUM | Checkpoint gate (FR25) interaction с review loop не специфицирован |
| **Guardrails (FR36-FR39)** | MEDIUM | 999-rules не тестируемы unit-тестами, только prompt quality |
| **Config (FR30-FR35)** | LOW | Straightforward, низкий risk |

### Mandatory AC для high-risk stories

| Область | Обязательные AC в stories |
|---------|--------------------------|
| **Review stories** | Формат findings (ЧТО/ГДЕ/ПОЧЕМУ/КАК); идемпотентность `[x]` + clear; isolation (no code writes) |
| **Session JSON parsing** | Golden files на edge cases (truncated JSON, unexpected fields, empty output) |
| **FR25 checkpoint gates** | Считает только `[x]` задачи, не execute attempts |
| **FR4 Smart Merge** | Backup sprint-tasks.md перед merge; golden file тесты merge-сценариев |
| **FR9 resume-extraction** | session_id capture + fallback при parse error |
| **FR26 CLAUDE.md** | Обновляет ТОЛЬКО секцию `## Ralph operational context`, не затрагивает остальное |
| **Prompt assembly** | Interface contract freeze: сигнатура `AssemblePrompt()` фиксируется после Epic 1, только internal changes |
| **Epic 3 integration** | Stub review step (mock "clean") + bridge golden file output как input runner'а |
| **Review prompts** | Adversarial golden files: bug-injection test + clean-code false positive resistance test per sub-agent |
| **Mutation Asymmetry** | Execute stories: AC "MUST NOT modify sprint-tasks.md". Review stories: AC "MUST NOT modify git working tree" |
| **Review atomicity** | `[x]` + clear findings = atomic operation (write both or neither). Epic 4 Story 5 |
| **Distillation backup** | Backup LEARNINGS.md before distillation overwrite. Epic 6 Story 3 |

### Epic Size Threshold Rule (Comparative Analysis)

> **Threshold:** Если Epic 3 при детальной декомпозиции (Step 2) превысит 13 stories → split:
> - **Epic 3a "Task Execution":** Scanner + Git client + Runner loop + Execute prompt + Retry (Stories 1-6). User value: "задачи выполняются"
> - **Epic 3b "Failure Recovery":** Resume-extraction + Emergency stops + Runner resume-on-rerun + Integration test (Stories 7-11). User value: "сбои обрабатываются"
> - Dependencies при split: Epic 4 → Epic 3a (runner loop). Epic 5 → Epic 3b (emergency gate upgrade). Epic 6 → Epic 3b (KnowledgeWriter)
>
> Текущая 6-epic структура подтверждена как оптимальная (score 65/75 vs альтернатив 43-60).

### Estimated Story Count

~45-55 stories для 42 MVP FR (вместо наивных ~30).

---
