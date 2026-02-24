# LLM-as-Judge в Ralph Loop: Исследование вариантов

**Дата:** 2026-02-24
**Контекст:** Выбор подхода к ревью кода в гибридной BMad+Ralph архитектуре
**Ограничение:** Без внешних API (GPT/Gemini) — только через Claude Code (подписка)

---

## Исследуемые варианты

| ID | Название | Суть |
|----|----------|------|
| **A** | Task tool sub-agents | Внутри Ralph-итерации спавним sub-agent(ов) через Task tool для ревью |
| **B** | Отдельная задача в sprint-tasks.md | Ревью как самостоятельная задача, выполняется в свежей Ralph-сессии |
| **C** | Тест-фикстура через Claude CLI | `claude -p "..."` вызывается как subprocess из теста (vitest/jest) |

---

## 1. Что принято в сообществе (февраль 2026)

### 1.1 Вариант A — самый распространённый

**Task tool sub-agents** стал де-факто стандартом в экосистеме Claude Code для ревью кода:

- **HAMY (февраль 2026)** — [9 параллельных sub-agent'ов](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents) для ревью: linter, code reviewer, security, quality, test quality, performance, dependency safety, simplification, и test runner. Результат: ~75% полезных находок (было <50%).

- **ralphex (umputun)** — [5 параллельных review-агентов](https://ralphex.com/) (quality, implementation, testing, simplification, documentation) + внешний GPT-5 codex review + финальный проход. Это единственная зрелая Ralph-реализация с встроенным multi-agent review.

- **Nick Tune / O'Reilly (2025)** — [Auto-Reviewing Claude's Code](https://www.oreilly.com/radar/auto-reviewing-claudes-code/) через Stop hook + PostToolUse hook. Субагент с "критическим мышлением" проверяет изменения автоматически.

- **Официальная документация Claude Code** — [subagents.app/agents/reviewer](https://subagents.app/agents/reviewer) и [claude-code plugins/code-review](https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md) — встроенный review через sub-agents.

### 1.2 Вариант B — используется в Ralph Loop нативно

Подход "ревью как отдельная задача" — это **родной Ralph-паттерн**:

- **ralphex** использует именно этот подход: после выполнения задачи запускается отдельная фаза ревью (Phase 2 → Phase 3 → Phase 4), каждая в свежей сессии.
- **Canonical Ralph** (snarktank/ralph) — review phase встроена в цикл как отдельный шаг.
- **Clayton Farr Playbook** — ревью как часть PROMPT_build.md guardrail rules, тесты должны проходить перед коммитом.

### 1.3 Вариант C — нишевый, но растущий

**Тест-фикстура через CLI** пока используется ограниченно:

- **Clayton Farr Playbook** — единственная зрелая реализация с `createReview()` в `src/lib/llm-review.ts`, но использует GPT/Gemini API, а не Claude CLI.
- **Voicetest** — [реальная реализация](https://dev.to/pld/using-claude-code-as-a-free-llm-backend-for-voice-agent-testing-40bg) `claude -p --output-format json` как subprocess для LLM-judge в тестах, с очисткой `ANTHROPIC_API_KEY` для использования подписки.
- **Attest Framework** — [8-слойный assertion pipeline](https://github.com/attest-framework/attest) с LLM-as-Judge на 6-м слое, интеграция с vitest.
- **Promptfoo** — [интеграция с Jest/Vitest](https://www.promptfoo.dev/docs/integrations/jest/) для LLM grading, semantic similarity.

---

## 2. Сравнительный анализ

### 2.1 Архитектурное соответствие Ralph Loop

| Критерий | A (Task tool) | B (Отдельная задача) | C (Тест-фикстура) |
|----------|:---:|:---:|:---:|
| Свежий контекст для ревью | Нет (общий контекст) | Да | Да |
| Ralph-philosophy aligned | Частично | Полностью | Ортогонально |
| Backpressure (блокирует прогресс) | Нет* | Да | Да (тест не пройдёт) |
| Автоматический | Да | Да | Да |

*Sub-agent может вернуть замечания, но не блокирует основной агент автоматически.

### 2.2 Качество ревью

| Критерий | A (Task tool) | B (Отдельная задача) | C (Тест-фикстура) |
|----------|:---:|:---:|:---:|
| Контекст кодовой базы | Полный (видит файлы) | Полный (свежая сессия) | Ограничен (только artifact) |
| Возможность запускать тесты | Нет (read-only) | Да | Частично (сама фикстура = тест) |
| Параллельность проверок | Да (несколько agents) | Да (несколько задач) | Да (несколько тестов) |
| Детерминированность | Нет | Нет | Нет |
| Повторяемость в CI | Нет | Нет | Да |

### 2.3 Стоимость и производительность

| Критерий | A (Task tool) | B (Отдельная задача) | C (Тест-фикстура) |
|----------|:---:|:---:|:---:|
| Потребление ходов из итерации | Да (1-3 хода на агент) | Нет (отдельная сессия) | Нет (subprocess) |
| Параллельный запуск | Да | Нет (sequential tasks) | Да (parallel tests) |
| Стоимость при подписке | Входит в итерацию | Отдельная сессия | `claude -p` по подписке |
| Задержка | ~30-60 сек | ~2-5 мин (полная сессия) | ~10-30 сек на тест |

### 2.4 Сложность реализации

| Критерий | A (Task tool) | B (Отдельная задача) | C (Тест-фикстура) |
|----------|:---:|:---:|:---:|
| **Сложность реализации** | **Низкая** | **Низкая** | **Средняя** |
| Что нужно написать | Промпт для sub-agent | Задача + промпт | Обёртка + промпт + парсер |
| Инфраструктура | Ничего (встроено) | Ничего (часть task list) | vitest/jest + CLI wrapper |
| Настраиваемость | Высокая (любой промпт) | Высокая | Очень высокая (код) |
| Отладка | Сложно (sub-agent) | Просто (отдельная сессия) | Просто (обычный тест) |
| LOC для MVP | ~20-50 (промпт) | ~20-50 (промпт) | ~100-200 (wrapper + тесты) |

---

## 3. Детальный разбор каждого варианта

### 3.1 Вариант A — Task tool sub-agents

**Как работает:**
```
# Внутри Ralph-итерации, после выполнения основной задачи:
Task(subagent_type="general-purpose", prompt="Review the changes in...")
```

**Плюсы:**
- Мгновенная обратная связь в той же сессии
- Параллельный запуск нескольких ревьюеров (HAMY: 9, ralphex: 5)
- Минимальная реализация — достаточно промпта
- Полный доступ к файловой системе
- Принятый паттерн в сообществе (самый популярный)

**Минусы:**
- Съедает ходы из бюджета итерации (основная задача получает меньше)
- Нет формального backpressure — ревью-замечания могут быть проигнорированы
- Не отделён от основной работы (тот же контекст, self-review bias)
- При Ralph loop: контекст уже загружен задачей, ревью работает в "хвосте"
- Не воспроизводим в CI

**Кто использует:** HAMY (9 agents), ralphex Phase 2 (5 agents), Nick Tune (Stop hook), Anthropic plugins

### 3.2 Вариант B — Отдельная задача в sprint-tasks.md

**Как работает:**
```markdown
# sprint-tasks.md
- [x] TASK-1: Implement auth module
- [ ] TASK-1-REVIEW: Review auth module implementation
- [ ] TASK-2: Implement user profile
```

**Плюсы:**
- Полностью соответствует Ralph-philosophy (свежий контекст)
- Полный бюджет ходов на ревью
- Можно запустить тесты, Serena, lint — всё что угодно
- Нет self-review bias (другая сессия, другой контекст)
- Используется в ralphex (Phase 2-4 отдельные сессии)

**Минусы:**
- Увеличивает количество итераций (каждая задача = 2 итерации)
- Не классический LLM-as-Judge (нет программного pass/fail)
- Задержка: полная сессия вместо быстрого sub-agent
- Может возникнуть "review loops" (review → fix → review → fix...)
- Нет интеграции с test suite

**Кто использует:** ralphex (Phase 2-4), canonical Ralph (review phase)

### 3.3 Вариант C — Тест-фикстура через Claude CLI

**Как работает:**
```typescript
// src/lib/llm-review.ts
export async function createReview({ criteria, artifact }: ReviewInput): Promise<ReviewResult> {
  const result = await execSync(
    `claude -p "${criteria}\n\nArtifact:\n${artifact}" --output-format json`
  );
  const parsed = JSON.parse(result);
  return { pass: parsed.pass, feedback: parsed.feedback };
}

// __tests__/auth.review.test.ts
test('auth module follows security best practices', async () => {
  const code = readFileSync('src/auth/index.ts', 'utf-8');
  const review = await createReview({
    criteria: 'Code follows OWASP top 10 security guidelines',
    artifact: code
  });
  expect(review.pass).toBe(true);
});
```

**Плюсы:**
- Классический LLM-as-Judge (как в Farr Playbook)
- Программный binary pass/fail — автоматический backpressure
- Запускается в CI (vitest/jest)
- Повторяемость и версионирование критериев
- Работает через подписку (`claude -p`, без API ключа)
- Проверен на практике (Voicetest)
- Интеграция с существующими тест-фреймворками

**Минусы:**
- Нужно написать обёртку (~100-200 LOC)
- Ограниченный контекст: видит только то, что передано в artifact
- Не может запускать команды или читать произвольные файлы
- Последовательный запуск (одна CLI-сессия за раз, rate limit)
- Не детерминистичен (тот же тест может дать разные результаты)
- Зависимость от CLI-формата вывода Claude
- Возможные rate limits подписки при массовом запуске

**Кто использует:** Voicetest (Claude CLI), Farr Playbook (GPT/Gemini API), Attest framework (8-layer pipeline), Promptfoo (Jest/Vitest)

---

## 4. Рекомендация для bmad-ralph

### 4.1 Для MVP

**Вариант A (Task tool sub-agents)** — лучший баланс для старта:
- Минимальная реализация (только промпт)
- Работает "из коробки" в Claude Code
- Самый популярный паттерн в сообществе
- Можно начать с 1-2 ревьюеров, потом масштабировать

### 4.2 Для Production

**Гибрид A+C** — рекомендуемая эволюция:
- **A** для быстрого in-loop ревью (sub-agents после каждой задачи)
- **C** для формального quality gate (тест-фикстуры в CI, перед коммитом)
- **B** для тяжёлых случаев (сложные задачи требующие глубокого review)

### 4.3 Обоснование

Сообщество в 2026 году ясно сходится на **sub-agent review** (вариант A) как основном паттерне. Однако исследование показывает, что это НЕ заменяет формальный quality gate — ralphex, например, использует и sub-agents (A), и отдельные review-фазы (B), и внешний codex review.

Вариант C — единственный, который даёт **программный backpressure** (тест не прошёл = итерация повторяется), что является ключевым принципом Farr Playbook. Но его сложнее реализовать и он ограничен в контексте.

---

## 5. Таблица источников

| # | Источник | Тип | Качество | URL |
|---|---------|-----|----------|-----|
| S1 | HAMY — 9 Parallel AI Agents | Блог-пост, февраль 2026 | A | [hamy.xyz](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents) |
| S2 | ralphex | Open-source проект | A | [ralphex.com](https://ralphex.com/) / [GitHub](https://github.com/umputun/ralphex) |
| S3 | Nick Tune — Auto-Reviewing Claude's Code | O'Reilly, 2025 | A | [oreilly.com](https://www.oreilly.com/radar/auto-reviewing-claudes-code/) |
| S4 | Clayton Farr Ralph Playbook | Open-source guide | A | [GitHub](https://github.com/ClaytonFarr/ralph-playbook) |
| S5 | Voicetest — Claude Code as LLM Backend | DEV Community | B | [dev.to](https://dev.to/pld/using-claude-code-as-a-free-llm-backend-for-voice-agent-testing-40bg) |
| S6 | Attest Framework | Open-source framework | A | [GitHub](https://github.com/attest-framework/attest) |
| S7 | Promptfoo — Jest/Vitest Integration | Документация | A | [promptfoo.dev](https://www.promptfoo.dev/docs/integrations/jest/) |
| S8 | Claude Code Subagents Docs | Официальная документация | A | [code.claude.com](https://code.claude.com/docs/en/sub-agents) |
| S9 | Agent-as-a-Judge (arXiv) | Научная статья, 2025 | A | [arxiv.org](https://arxiv.org/html/2508.02994v1) |
| S10 | From Code to Courtroom (arXiv) | Survey paper, 2025 | A | [arxiv.org](https://arxiv.org/html/2510.24367v1) |
| S11 | LLM as a Judge 2026 Guide | Руководство | B | [labelyourdata.com](https://labelyourdata.com/articles/llm-as-a-judge) |
