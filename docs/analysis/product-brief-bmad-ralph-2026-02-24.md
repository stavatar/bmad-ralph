---
stepsCompleted: [1, 2, 3, 4, 5]
inputDocuments:
  - docs/analysis/brainstorming-session-2026-02-23.md
  - docs/research/llm-as-judge-variants-research-2026-02-24.md
  - docs/research/research-1-serena-ralph-2026-02-24.md
  - docs/research/research-2-tdd-vs-prewritten-tests-2026-02-24.md
  - docs/research/research-3-e2e-in-ralph-2026-02-24.md
  - docs/research/research-4-review-agents-composition-2026-02-24.md
  - docs/research/research-5-human-gates-2026-02-24.md
workflowType: 'product-brief'
lastStep: 5
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-02-24'
---

# Product Brief: bmad-ralph

**Date:** 2026-02-24
**Author:** Степан

---

## Executive Summary

**bmad-ralph** — open-source CLI-утилита, которая соединяет структурированное планирование BMad Method v6 с автономным выполнением через Ralph Loop. Утилита берёт BMad stories с acceptance criteria, конвертирует их в задачи через детерминированный bridge, и автономно выполняет в bash-цикле с fresh context на каждой итерации. Встроенные параллельные review-агенты (4 шт.), трёхуровневое извлечение знаний, интеграция с Serena для token economy, и гибкая система human gates — от полной автономии до ручного контроля каждой задачи.

Целевая аудитория: разработчики, использующие Claude Code для автономной разработки и желающие масштабировать AI-driven workflow за пределы одной сессии.

---

## Core Vision

### Problem Statement

Текущие подходы к AI-driven разработке страдают от трёх ключевых проблем:

1. **Переполнение контекста.** Длинные сессии Claude Code приводят к compaction, деградации качества и потере знаний. Sprint-run вызывает 2-4 инцидента compaction за спринт.
2. **Ручная работа между итерациями.** Разработчик вынужден вручную запускать агента, передавать контекст, проверять результат, запускать снова — десятки раз за epic.
3. **Жёсткость оркестрации.** Skill-based подход (sprint-run, 641 LOC, 4 скилла) захардкожен под конкретный pipeline и не адаптируется под произвольные задачи.

### Problem Impact

- Разработчик тратит 30-50% времени на "обслуживание" AI вместо продуктовой работы
- Качество кода деградирует к концу длинных сессий
- Масштабирование невозможно: 100 задач = 100 ручных запусков
- Знания, полученные агентом в одной сессии, теряются в следующей

### Why Existing Solutions Fall Short

| Решение | Что хорошо | Чего не хватает |
|---------|-----------|----------------|
| **Canonical Ralph** | Простота, fresh context | Нет планирования, нет review, нет bridge |
| **ralphex** | 5 review-агентов, полный pipeline | Нет BMad-интеграции, свой формат плана |
| **bmalph** | BMad bridge, Smart Merge | Продолжает в той же сессии (нарушает Ralph), нет извлечения знаний, нет Serena |
| **Farr Playbook** | Минимализм, 999-guardrails, LLM-as-Judge | Нет bridge из stories, нет формального планирования |

Ни одно решение не объединяет **структурированное планирование + автономное выполнение + review + knowledge extraction + token economy**.

### Proposed Solution

**bmad-ralph** — CLI-утилита с тремя ключевыми компонентами:

1. **Bridge** (`ralph bridge`): Детерминированный код-конвертер читает BMad stories → генерирует `sprint-tasks.md` с AC-derived test cases, human gates, и служебными задачами. Smart Merge при повторном запуске.

2. **Loop** (`ralph run`): Bash-цикл с fresh context на каждой итерации. Full Serena index на старте + incremental перед каждой итерацией. Agent сам читает sprint-tasks.md, выбирает задачу, реализует, запускает тесты. ATDD-lite: тесты пишутся первыми на основе AC.

3. **Review & Learn**: 4 параллельных review sub-агента (quality, implementation, simplification, test-coverage) после каждой задачи. Трёхуровневое извлечение знаний (непрерывное + дистилляция + финальное). Resume-extraction при неуспехе.

**Режимы работы:**
- `ralph run` — полная автономия (без human gates)
- `ralph run --gates` — human gates на user-visible milestones
- `ralph run --gates --every 5` — периодические checkpoints каждые N задач

**Human gate actions:**
- **approve** (Enter) — продолжить
- **retry** (r) — feedback → fix-задача вперёд (MVP), rollback (post-MVP)
- **correct** (c) — Claude+BMad правит оригинальную story → автоматический re-bridge (MVP last epics)
- **skip** (s) — пропустить задачу
- **quit** (q) — остановить loop

### Key Differentiators

1. **BMad + Ralph в одном инструменте.** Единственное решение, где AI планирует через BMad (PRD → Architecture → Stories) и выполняет через Ralph (fresh context, bash loop).
2. **Guardrails из Farr Playbook.** 999-series правила и промпт-структура для надёжного автономного выполнения.
3. **Serena интеграция.** Token economy через semantic code retrieval — ни одна Ralph-реализация этого не имеет.
4. **Гибкий контроль.** От "запустил и ушёл на 100 задач" до "проверяю каждый экран" — одним флагом.
5. **Correct flow.** При несогласии на human gate — правка оригинальной BMad story через Claude + автоматический re-bridge. Source of truth сохраняется.
6. **Knowledge extraction.** Три уровня: непрерывный (Farr-стиль), дистилляция паттернов, финальный анализ спринта. Resume-extraction при неуспехе.
7. **Test coverage validation.** Четвёртый review-агент проверяет что каждый AC покрыт тестом — нет "зелёного нуля".

### Open Questions (для Phase 2 — Architecture)

- **Стек bridge**: bash, TypeScript, или Python? (Winston: boring technology → bash)
- **Quick start без BMad**: `ralph run --plan plan.md` для тех, кто не использует BMad (post-MVP)

---

## Target Users

### Primary Users

**"Алекс" — Solo/Mid-Senior разработчик с Claude Code**

- **Профиль:** Mid-to-Senior разработчик (3-8+ лет опыта), работает solo или в маленькой команде. Подписка Claude Pro/Max. Активно использует Claude Code как основной инструмент разработки.
- **Контекст:** Строит web-приложения (full-stack), side-projects, SaaS-продукты или фриланс. Привык к высокой автономии, ценит инструменты которые экономят время.
- **Боль:** Устал от ручного "кормления" Claude — запустил, подождал, проверил, скопировал контекст, запустил снова. На длинных задачах (10+ файлов, epic на 2 дня) контекст деградирует, приходится перезапускать, теряя накопленные знания.
- **Текущие workarounds:** Либо vanilla Ralph loop (простой, но без планирования и review), либо ручная оркестрация через Claude Code skills, либо просто терпит compaction.
- **Success moment:** Запустил `ralph run`, ушёл на обед, вернулся — 8 задач выполнено, тесты зелёные, осталось approve один human gate на новом экране.
- **Мотивация:** "Хочу чтобы AI делал рутину, а я принимал решения."

### Secondary Users

N/A для MVP. В будущем возможны:
- PM/аналитик, который создаёт stories через BMad, а dev запускает `ralph run`
- Контрибьюторы open-source проекта bmad-ralph

### User Journey

1. **Discovery:** Находит bmad-ralph через GitHub, русскоязычные dev-сообщества, или рекомендации в Ralph/BMad community
2. **Onboarding:** Установка + имеющиеся BMad stories → `ralph bridge` → `ralph run` — первый результат за 10-15 минут
3. **Core Usage:** Каждый sprint: создал stories через BMad → `ralph bridge` → `ralph run --gates` → approve на human gates → done
4. **Aha-moment:** Первый раз когда 5+ задач выполнены автономно с зелёными тестами и quality review — без единого ручного вмешательства
5. **Long-term:** bmad-ralph становится стандартным workflow: BMad планирует, Ralph выполняет. Накопленные LEARNINGS.md улучшают качество от спринта к спринту

---

## Success Metrics

### User Success

| Метрика | Целевое значение | Как измеряем |
|---------|-----------------|--------------|
| **Автономное выполнение задач** | 10-20 задач за один `ralph run` без вмешательства | Счётчик итераций между human gates |
| **Качество correct flow** | После коррекции курс меняется в нужную сторону с первой попытки в >80% случаев | Количество повторных correct на ту же проблему |
| **Тесты зелёные после итерации** | >90% итераций завершаются с green tests | loop.sh статистика |
| **Экономия времени** | 3-5x по сравнению с ручной оркестрацией | Субъективная оценка пользователя |
| **Review полезность** | >50% замечаний review-агентов действительно ценные | Субъективная оценка |

**Ключевой критерий:** Не количество задач без остановки, а **качество коррекции курса**. Страшно не то, что human gate сработал, а то, что после feedback система не может перестроиться.

### Business Objectives (Open-Source)

| Метрика | 3 месяца | 12 месяцев |
|---------|----------|------------|
| **GitHub Stars** | 100+ | 1000+ |
| **Уникальные пользователи** | 10-20 активных | 100+ активных |
| **Community вклад** | Первые issues/PRs от сторонних людей | Регулярные контрибьюции, 5+ contributors |
| **Упоминания** | Посты в русскоязычных dev-сообществах | Упоминания в англоязычных блогах/ютуб |

### Key Performance Indicators

1. **Task completion rate** — % задач, выполненных автономно (без retry/correct) за спринт. Цель: >70%.
2. **Correct-to-resolution** — после коррекции курса (correct flow), сколько итераций до нужного результата. Цель: 1-2 итерации.
3. **Knowledge retention** — паттерны из LEARNINGS.md реально применяются в последующих итерациях. Качественная метрика.
4. **Onboarding time** — от `git clone` до первого успешного `ralph run`. Цель: <15 минут.

---

## MVP Scope

### Core Features

**1. `ralph bridge` — Конвертер BMad → Ralph**
- Детерминированный код-конвертер: читает BMad story-файлы → генерирует `sprint-tasks.md`
- AC-derived test cases: acceptance criteria из stories конвертируются в тестовые требования в задачах
- Разметка human gates на user-visible milestones (новая функция, экран, кнопка, работающий flow)
- Генерация служебных задач (reindex, review, meta-analysis, finish)
- Smart Merge при повторном запуске: обновляет существующий `sprint-tasks.md` без потери статуса выполненных задач

**2. `ralph run` — Автономный execution loop**
- Bash while-loop с fresh context на каждой итерации (никакого compaction)
- Serena integration (best effort, не блокирует loop): full index на старте + incremental перед каждой итерацией
- Агент сам читает `sprint-tasks.md`, выбирает задачу, реализует, запускает тесты
- ATDD-lite: тесты пишутся первыми на основе AC-derived test cases из задачи
- Completion check: тесты green + review passed
- MAX_ITER как простой safety limit
- Режимы: `ralph run` (полная автономия), `ralph run --gates` (human gates на milestones), `ralph run --gates --every N` (периодические checkpoints)

**3. Review sub-агенты (4 параллельных)**
- **quality** (sonnet) — баги, security, race conditions, error handling
- **implementation** (sonnet) — код решает задачу, AC выполнены, нет scope creep
- **simplification** (haiku) — over-engineering, мёртвый код, дублирование
- **test-coverage** (sonnet) — каждый AC покрыт тестом, нет "зелёного нуля"
- Запуск параллельно через Task tool после каждой задачи
- **Review failure flow:** Critical finding = итерация не пройдена (аналог красных тестов). Агент исправляет в рамках оставшихся turns. Если turns исчерпаны — resume-extraction с выводами из review → следующая итерация подхватывает ту же задачу с контекстом ошибок

**4. Human Gates**
- Approve (Enter) — продолжить
- Retry (r) — feedback → fix-задача вперёд очереди
- Skip (s) — пропустить задачу
- Quit (q) — остановить loop
- Дефолтные точки: после bridge, первая задача epic'а, user-visible UI milestones, перед merge

**5. Knowledge Extraction (два уровня)**
- Непрерывная запись: агент пишет в AGENTS.md + LEARNINGS.md по ходу каждой итерации (Farr-стиль)
- Resume-extraction: при неуспешной итерации — `--resume` той же сессии для извлечения знаний (2-3 turns), включая выводы из Critical Review findings

**6. Guardrails**
- 999-Series правила из Farr Playbook для надёжного автономного выполнения
- PROMPT_build.md с инструкциями агенту: правила записи знаний, test-first, scope discipline

### MVP Phase 2 (после стабилизации основного loop)

| Компонент | Зависимость |
|-----------|-------------|
| **Correct flow** (c) — правка BMad story → re-bridge | Требует stable bridge + run + human gates |
| **Meta-task дистилляция** — сжатие LEARNINGS.md в паттерны каждые N итераций | Требует накопленные данные в LEARNINGS.md |

### Out of Scope for MVP

| Компонент | Причина отложить |
|-----------|-----------------|
| **Rollback retry** (git reset + повтор) | Fix-forward достаточен; rollback требует надёжной git-автоматизации |
| **Circuit breaker** (CLOSED/HALF_OPEN/OPEN) | Простой MAX_ITER покрывает safety |
| **Quick start без BMad** (`ralph run --plan plan.md`) | Фокус на BMad-интеграции |
| **LLM-as-Judge test fixtures** (вариант C — `claude -p`) | Task tool sub-agents достаточны для MVP |
| **Notifications** (Slack/email/desktop) | Loop интерактивный |
| **CI/CD интеграция** | Не нужна для solo developer workflow |
| **Performance review agent** | Нечего оптимизировать на этапе MVP |
| **Полная BMad CLI интеграция** | Фазы 1-3 вручную через Claude Code + BMad skills |

### MVP Success Criteria

| Критерий | Порог | Как проверить |
|----------|-------|---------------|
| **End-to-end flow работает** | BMad stories → bridge → run → задачи выполнены с green tests | Smoke test на реальном проекте |
| **Автономность** | 10+ задач за один `ralph run` без вмешательства | Счётчик итераций |
| **Качество коррекции** | После retry курс меняется правильно с первой попытки в >80% случаев | Ручная проверка |
| **Review полезность** | >50% замечаний ценные (MVP порог), цель >70% к production | Субъективная оценка |
| **Review → fix loop** | Critical findings исправляются за 1-2 дополнительные итерации | Статистика loop |
| **Onboarding** | От clone до первого успешного `ralph run` за <15 минут | Таймер |
| **Knowledge retention** | LEARNINGS.md содержит actionable паттерны после спринта | Ручная проверка |

### Future Vision

**Ближайшее (post-MVP):**
- Rollback retry: git reset + повтор задачи с feedback
- Circuit breaker: автоматическая остановка при серии неуспехов
- LLM-as-Judge test fixtures: формальный quality gate в CI через `claude -p`
- Quick start: `ralph run --plan plan.md` для пользователей без BMad

**Среднесрочное (6-12 месяцев):**
- Полная BMad CLI интеграция: `ralph init` → `ralph plan` → `ralph bridge` → `ralph run`
- Notifications: desktop/Slack при human gate, завершении спринта
- Performance и security review agents
- Dashboard с метриками спринта

**Долгосрочное (12+ месяцев):**
- Multi-agent parallelism: несколько задач одновременно на разных ветках
- Team mode: несколько разработчиков, общий sprint-tasks.md
- Plugin system: кастомные review agents, bridge adapters, notification providers
- Интеграция с другими planning-фреймворками
