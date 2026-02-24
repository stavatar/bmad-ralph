# Research 2: TDD vs Pre-written Tests в Ralph Loop

**Дата:** 2026-02-24
**Вопрос:** Как стыкуются TDD и Ralph цикл? Отдельные stories на тесты или AC в story-файлах?

---

## 1. Проблема

В текущем sprint-run используется TDD Gate 4 (тесты обязательно перед кодом). В Ralph loop каждая итерация = fresh context. Как обеспечить test-first подход, когда агент может "забыть" о TDD?

## 2. Что говорит сообщество

### 2.1 TDD с AI агентами — "dramatically better results"

[ThoughtWorks Agile Workshop (февраль 2026)](https://www.theregister.com/2026/02/20/from_agile_to_ai_anniversary/):
> "TDD produces dramatically better results from AI coding agents"

**Ключевая проблема без TDD:** AI агент может "cheating" — написать тест, который просто подтверждает его же сломанную реализацию. Тесты становятся circular validation.

**С TDD:** тесты = фиксированные требования, которые агент ДОЛЖЕН удовлетворить. Не может переопределить цель.

### 2.2 Практика в Ralph Loop

[Clayton Farr Playbook](https://claytonfarr.github.io/ralph-playbook/) — **Tests before code**:
- Acceptance criteria в спеке → test requirements в implementation plan
- Агент ЗНАЕТ тесты до начала кода
- Completion signal: required tests pass (детерминированный) vs "seems done?" (вероятностный)

[Ralph community](https://agentfactory.panaversity.org/docs/General-Agents-Foundations/general-agents/ralph-wiggum-loop):
- Тесты = mandatory completion criteria
- Агент обязан написать и пройти тесты перед тем, как система примет выход
- Для субъективных AC → LLM-as-Judge тесты

### 2.3 Три подхода к тестам в AI-driven разработке

| Подход | Суть | Когда работает | Проблема |
|--------|------|---------------|---------|
| **Strict TDD** (Red-Green-Refactor) | Один тест → fail → implement → pass → next test | Хорошо определённый API, unit-level | Медленно с AI; агент часто пытается написать код первым |
| **ATDD** (Acceptance Test-Driven) | Все acceptance-тесты написаны до кода | Story-level, интеграционный | Требует хорошие AC в story; тесты могут быть хрупкими |
| **Test-alongside** | Агент пишет тесты параллельно с кодом | Exploratory, UI, прототипы | Circular validation — агент подстраивает тесты под код |

### 2.4 Реальный опыт (2025-2026)

**Проблема adoption:** команды часто бросают TDD с AI к 5-му дню — "слишком медленно". Но после 10,000 LOC TDD окупается (меньше багов, быстрее итерации).

**Рекомендация сообщества:** "agentic TDD is expensive upfront and cheap long-term"

## 3. Анализ для bmad-ralph

### 3.1 ATDD — лучший fit для Ralph Loop

Strict TDD (Red-Green-Refactor) **плохо ложится** на Ralph:
- Требует множество micro-итераций в ОДНОЙ сессии
- Ralph = одна задача за итерацию, fresh context
- Red-Green-Refactor loop внутри итерации = потеря контекста при следующей

**ATDD** ложится идеально:
- BMad story содержит AC → конвертируется в test requirements
- Код-конвертер генерирует задачу с embedded test criteria
- Агент в итерации: видит задачу + test criteria → пишет тесты + код → runs → pass/fail
- Completion signal: все AC-тесты проходят

### 3.2 Предлагаемый формат задачи

```markdown
## TASK-3: Implement authentication middleware

**Story ref:** stories/auth-middleware.md
**AC-derived tests:**
- [ ] Test: POST /login with valid credentials returns 200 + JWT token
- [ ] Test: POST /login with invalid credentials returns 401
- [ ] Test: Protected routes return 403 without token
- [ ] Test: Expired token returns 401 with "token expired" message

**Implementation notes:**
- Use argon2 for password hashing
- JWT expiry: 24h
```

Агент видит конкретные тесты, которые ДОЛЖНЫ быть написаны и пройдены. Не может "подстроить" тесты.

### 3.3 Где НЕ нужен test-first

| Тип задачи | Test-first? | Почему |
|------------|:-----------:|--------|
| Business logic, API | Да | Детерминированный, testable |
| UI/UX, стили | Нет → LLM-as-Judge | Субъективно |
| Config, env setup | Нет | Smoke test достаточно |
| Refactoring | Да (существующие тесты) | Regression protection |
| Documentation | Нет | Нет кода |

## 4. Рекомендация

### MVP: ATDD-lite (два уровня)

**Уровень 1 — На этапе Create Story (BMad Phase 2):**
- При создании story через BMad workflow `create-story` добавить в промпт указание:
  - Формулировать AC так, чтобы они легко конвертировались в тесты
  - Добавлять конкретные test cases к каждому AC где возможно
  - Пример: вместо "User can log in" → "POST /login with valid email+password returns 200 + JWT; POST /login with wrong password returns 401 with error message"
- Это обеспечивает quality на входе — код-конвертеру останется только скопировать

**Уровень 2 — На этапе кода (Ralph Loop):**
1. **Код-конвертер** при генерации `sprint-tasks.md` берёт AC + test cases из story → добавляет как "AC-derived tests" в задачу
2. **PROMPT_build.md** содержит правило: "Write tests FIRST based on AC-derived tests, then implement. Tests must pass before marking task complete."
3. **Не Strict TDD** — не требуем Red-Green-Refactor micro-cycle, но требуем test-first на уровне задачи
4. **Completion check** в loop.sh: `npm test` / `vitest run` должен быть green

### Production: ATDD + LLM-as-Judge

- Объективные AC → ATDD (vitest)
- Субъективные AC → LLM-as-Judge тест-фикстуры (вариант C из решения 9)
- Regression suite растёт с каждым спринтом

## Источники

- [TDD ideal for AI (The Register, Feb 2026)](https://www.theregister.com/2026/02/20/from_agile_to_ai_anniversary/)
- [Ralph Playbook — test strategy](https://claytonfarr.github.io/ralph-playbook/)
- [TDD Paradigm Shift with Claude Code](https://medium.com/@moradikor296/the-tdd-paradigm-shift-why-test-driven-development-is-claude-codes-killer-discipline-9be9616d79f6)
- [AI Hero — TDD Skill for Claude Code](https://www.aihero.dev/skill-test-driven-development-claude-code)
- [ATDD overview](https://blog.logrocket.com/product-management/acceptance-test-driven-development/)
- [Ralph Loop test strategy](https://agentfactory.panaversity.org/docs/General-Agents-Foundations/general-agents/ralph-wiggum-loop)
