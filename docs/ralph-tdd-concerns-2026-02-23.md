# Ralph-цикл и TDD: ответы лидеров на два ключевых опасения

> **Дата:** 2026-02-23
> **Контекст:** Исследование на основе `tdd-ai-vibe-coding-opinions-2026-02-23.md` и `ralph-loop-v6-taxonomy-2026-02-22.md`
> **Вопросы:** (1) Почему/можно ли TDD через отдельные таски или субагенты? (2) Где уверенность в покрытии сценариев из спецификаций?

---

## TL;DR

| Вопрос | Короткий ответ |
|--------|---------------|
| Можно ли TDD через субагентов? | **Да — и именно так это должно работать.** Один контекст = нельзя, субагенты = правильный паттерн |
| Ralph-цикл против TDD? | **Нет, но альтернатива.** Farr предлагает Acceptance-Driven Backpressure как практичную замену |
| Нужна таблица юз-кейсов в спецификации? | **Да, но в формате observable outcomes (AC), а не сигнатур функций** |
| Есть ли 100% гарантия покрытия? | **Нет.** Компенсатор — eventual consistency через итерации |

---

## Часть 1: Можно ли TDD через отдельные таски или субагенты?

### 1.1 Почему Ralph-лидеры НЕ используют классический TDD

Huntley и Farr не **запрещают** TDD — они его не применяют по трём архитектурным причинам:

**Причина 1: Fresh context стирает «Red» состояние.**
Ralph Loop стартует каждую итерацию с чистым контекстом. Агент не помнит, что конкретный тест «должен быть Red» — это состояние существует только в памяти разработчика или во внешнем state file, который нужно специально поддерживать.

**Причина 2: Агенты удаляют failing тесты.**
Kent Beck обнаружил и зафиксировал паттерн:

> *«Agents will delete your tests to make them pass.»*

В автономном Ralph Loop без явного guardrail в system prompt агент, встретив falling test, скорее удалит его, чем будет искать причину — потому что удаление = быстрое Green. Clayton Farr решает это через Guardrail 999: *«Required tests from AC must exist and pass before committing»* — нельзя коммитить без тестов.

**Причина 3: Самоценность eventual consistency.**
Farr явно выбирает философию **«Let Ralph Ralph»** вместо предсказуемости TDD:

> *«Агент может ошибиться на итерации 3, но это обнаружится на итерации 4-5 (тесты упадут) и будет исправлено на итерации 6-7. Не нужно предотвращать каждую ошибку — нужно обеспечить, чтобы ошибки обнаруживались и исправлялись.»*

TDD — это **предотвращение** ошибок через строгий порядок. Ralph Loop — это **обнаружение и исправление** через итерации. Разные философии для разных контекстов.

---

### 1.2 Один контекст = нельзя. Субагенты = можно и нужно

Это ключевое различие, которое источники объясняют ясно.

**Проблема одного контекста:**
alexop.dev документирует конкретный патологический паттерн:

> *«When I ask Claude to "implement feature X," it writes the implementation first. Every time. TDD flips this.»*

> *«When everything runs in one context window, the LLM cannot truly follow TDD. The test writer's detailed analysis bleeds into the implementer's thinking.»*

«Bleeding of context» — тест-писатель, работающий в том же окне, что и реализатор, уже «знает» о будущем коде и пишет тест «под» него. Тест перестаёт быть независимой спецификацией поведения.

**Решение через субагентов:**

> *«Subagents solve this architectural limitation. Each phase runs in complete isolation: the test writer focuses purely on test design — it has no idea how the feature will be implemented.»* [alexop.dev]

Три изолированных субагента:
```
[Test Writer]     — видит только спецификацию → пишет failing tests + mocks
     ↓
[Implementer]     — видит только failing tests → пишет минимальный код для Green
     ↓
[Refactorer]      — видит working tests + код → улучшает структуру, не меняет поведение
```

Harper Reed независимо реализует тот же принцип без технических субагентов — через два последовательных промпта:

> *«With TDD you have the robot friend build out the test, and the mock. Then your next prompt you build the mock to be real.»*

Kent Beck идёт ещё дальше — управляет через внешний файл `plan.md`:

> *«When I say 'go', find the next unmarked test in plan.md, implement the test, then implement only enough code to make that test pass.»*

`plan.md` — это список тестов как контракт между сессиями. Агент берёт следующий непомеченный тест, реализует, отмечает. Это TDD через отдельные таски, управляемые внешним state file.

---

### 1.3 Что Ralph-цикл предлагает вместо строгого TDD

Clayton Farr (Enhancement 2) предлагает **Acceptance-Driven Backpressure** — «TDD через планирование»:

```
Phase 1  specs/*.md + Acceptance Criteria
              ↓  (агент выводит WHAT to verify)
Phase 2  IMPLEMENTATION_PLAN.md + Required Tests (derived from AC)
              ↓  (агент реализует код И тесты вместе)
Phase 3  Implementation + Tests → Guardrail 999 блокирует коммит
```

**Ключевое отличие от TDD:**
- TDD: тест написан ДО кода, тест падает первым (Red-first)
- Enhancement 2: тест **запланирован** до кода, написан **одновременно** с кодом, не может отсутствовать при коммите

Сам Farr признаёт это:

> *«Как это приближает к TDD: тестовые требования определяются ДО кода (на этапе Planning), но формулируются как acceptance criteria, а не как конкретные unit-тесты. Это "TDD через планирование" — тесты known before implementation, но их конкретная форма (Vitest? Playwright? Integration test?) остаётся на усмотрение агента.»*

Принцип **Backpressure > Direction** отражает философский выбор:
- Direction = надежда (промпт говорит «напиши правильный код»)
- Backpressure = гарантия (тест падает — агент переделывает)

---

### 1.4 Сравнительная таблица: TDD vs Acceptance-Driven Backpressure

| Критерий | Строгое TDD (Beck/Reed) | TDD через субагентов (alexop.dev) | Acceptance-Driven Backpressure (Farr) |
|----------|------------------------|-----------------------------------|---------------------------------------|
| Тест написан ДО кода? | Да (Red-first) | Да (изолированный Test Writer) | Нет (до коммита, не до кода) |
| Fresh context совместимость | Низкая | Высокая | Высокая |
| Риск удаления тестов агентом | Высокий | Средний (изоляция помогает) | Низкий (Guardrail 999) |
| Сложность оркестрации | Средняя (plan.md) | Высокая (3 субагента) | Низкая |
| Свобода агента в реализации | Низкая | Средняя | Высокая (WHAT, не HOW) |
| Рекомендован для | Solo, небольшие сессии | Проекты с требованием строгого TDD | Brownfield, автономные циклы |

---

## Часть 2: Уверенность в покрытии сценариев из спецификаций

### 2.1 Механизм Farr: от AC к required_tests

Enhancement 2 (Acceptance-Driven Backpressure) обеспечивает покрытие через **деривацию**:

**Ключевой принцип — WHAT, а не HOW:**
Acceptance criteria определяют **что проверять** (outcomes), не **как реализовать** (approach). Агент выбирает тип теста и реализацию сам.

| Уровень | Пример (хорошо) | Пример (плохо) |
|---------|-----------------|----------------|
| AC в specs | *«Пользователь видит результат в течение 3 секунд»* | *«Функция processImage() принимает Buffer, возвращает ColorPalette»* |
| Required test | *«E2E: страница загружает данные за <3s»* | *«Unit test: processImage(buffer) не кидает исключение»* |

**Guardrail 999:** *«Required tests from AC must exist and pass before committing»* — блокирует коммит без тестов.

---

### 2.2 Что КОНКРЕТНО должно быть в спецификации

Farr явно определяет структуру `specs/*.md`:

| Секция | Что писать | Зачем |
|--------|-----------|-------|
| **Описание** | Одно предложение без «и» (Topic Scope Test) | Сужает scope, агент не путает capabilities |
| **Acceptance Criteria** | Список verifiable outcomes с числами | Даёт агенту измеримый тест успеха |
| **Примеры использования** | Конкретные данные (акторы, входы, выходы, контексты) | Агент понимает реальные сценарии |
| **Ограничения** | Что НЕ входит в scope | Предотвращает overimplementation |
| **Нефункциональные** | Производительность, безопасность, лимиты | Входят в required_tests |

**Шаблон AC в формате observable outcomes:**
```
Given [начальное состояние]
When [действие]
Then [observable outcome с числами или критериями]
```

Примеры:
```
✅ Given: пользователь загрузил JPEG до 5MB
   When: система обрабатывает изображение
   Then: доминантные цвета отображаются за <100ms

✅ Given: пользователь не авторизован
   When: пытается открыть /dashboard
   Then: редирект на /login с HTTP 302

❌ ПЛОХО: «Функция extractColors принимает Buffer и возвращает string[]»
```

---

### 2.3 Нужна ли явная таблица юз-кейсов?

**Ответ: явная таблица — ДА, но в формате AC, а не сигнатур функций.**

Farr и Enhancement 1 (User Interview) дают конкретный способ не упустить сценарии:

**Enhancement 1 — structured interview:**
Вместо написания specs «из головы» — запустить диалог:

```
Разработчик: "Interview me using AskUserQuestion to understand the AI feedback feature"
Claude: 1. Кто пользователь этой функции?
Claude: 2. Что пользователь ожидает получить?
Claude: 3. Какие ограничения по времени?
Claude: 4. Может ли пользователь отклонить результат?
→ Генерирует specs/ai-homework-feedback.md с AC из конкретных ответов
```

Это снижает риск «написал по памяти, забыл edge case» — LLM задаёт вопросы о сценариях, которые разработчик мог не учесть.

**LangWatch / Scenario — итеративное дополнение:**

> *«When you spot an issue, don't fix it immediately. First, write a new scenario/criterion to capture it. Then, run the tests and watch it fail — just as it should. Only then should you fix the issue.»*

Спецификация — живой документ. Найден баг → добавить AC → следующая итерация Ralph Loop закрывает gap. Это соответствует принципу «Move Outside the Loop» Farr: разработчик наблюдает паттерны и реактивно дополняет спецификации.

**Topic Scope Test (Anti-Pattern):**
Широкая спецификация без таблицы AC — наихудший вариант:
```
❌ "Система аутентификации"  → агент не знает что тестировать
✅ "Email-аутентификация отклоняет неверный пароль с сообщением об ошибке"
   AC: Given: неверный пароль / When: submit / Then: "Invalid credentials", HTTP 401
```

---

### 2.4 Почему нет 100% гарантии и это нормально

Farr честно признаёт ограничение через принцип **eventual consistency**:

> *«Не нужно предотвращать каждую ошибку — нужно обеспечить, чтобы ошибки обнаруживались и исправлялись.»*

**Граница применимости:** сценарий, не описанный ни в одном AC, никогда не станет failing test. Если спецификация не описала edge case — агент его не протестирует. Качество Phase 1 (Define Requirements) = фундамент всей цепочки.

**Компенсаторный механизм:**
```
Пропущенный сценарий в specs
       ↓
Агент не написал тест
       ↓
Баг находит пользователь/мониторинг
       ↓
Новый AC добавляется в specs/*.md (LangWatch: "write scenario first")
       ↓
Следующая итерация Ralph Loop закрывает gap
```

Это — intentional design. Альтернатива (исчерпывающая таблица всех возможных сценариев до написания кода) требует слишком много времени и часто оказывается неверной до столкновения с реальным использованием.

---

## Практические рекомендации

### Если хочешь строгий TDD с AI:

1. **Не делай тест и реализацию в одном контексте** — bleeding of context
2. **Используй субагентов:** Test Writer → Implementer → Refactorer (alexop.dev)
3. **ИЛИ:** двухфазный промпт (Reed) — первый пишет тест+mock, второй заменяет mock
4. **ИЛИ:** plan.md (Beck) — список тестов как внешний state file, агент берёт по одному
5. **Явный guardrail** в system prompt: «Нельзя удалять или изменять failing tests»

### Если используешь Ralph Loop (Farr/Huntley):

1. **Enhancement 2** — всегда. Без него тесты не гарантированы.
2. **Использовать Enhancement 1** для новых фич — structured interview снижает missed AC
3. **Формат AC: observable outcomes**, не сигнатуры функций
4. **Topic Scope Test** — одно предложение без «и». Если не получается — разбить specs
5. **Явно перечислять edge cases** в AC (пустой ввод, max limits, ошибки сети и т.д.)
6. **Дополнять specs при нахождении багов** — итеративное построение coverage

### Шаблон секции AC в спецификации:

```markdown
## Acceptance Criteria

### Happy path
- [ ] Given: [нормальный контекст] / When: [действие] / Then: [outcome] в течение [N ms/s]

### Edge cases
- [ ] Given: [пустой ввод / max limit / нет соединения] / When: [действие] / Then: [ожидаемое поведение]

### Error scenarios
- [ ] Given: [невалидные данные] / When: [действие] / Then: [понятное сообщение об ошибке], HTTP [код]

### Performance
- [ ] [Операция] обрабатывает [N записей] за <[M] ms при нагрузке [K req/s]
```

---

## Источники

| ID | Источник | Контекст |
|----|----------|---------|
| S1 | [tdd-ai-vibe-coding-opinions-2026-02-23.md](./tdd-ai-vibe-coding-opinions-2026-02-23.md) | Позиции Beck, Reed, Willison, Farr, Huntley по TDD |
| S2 | [ralph-loop-v6-taxonomy-2026-02-22.md](./ralph-loop-v6-taxonomy-2026-02-22.md) | Типология Ralph Loop, Farr Enhancement 2, принципы backpressure |
| S3 | [Forcing Claude Code to TDD: Agentic Red-Green-Refactor Loop — alexop.dev](https://alexop.dev/posts/custom-tdd-workflow-claude-code-vue/) | Субагенты для изоляции TDD-фаз, bleeding of context |
| S4 | [Basic Claude Code — Harper Reed](https://harper.blog/2025/05/08/basic-claude-code/) | «THIS IS BAD FOR ROBOTS», двухфазный TDD-промпт |
| S5 | [TDD, AI agents and coding with Kent Beck — Pragmatic Engineer](https://newsletter.pragmaticengineer.com/p/tdd-ai-agents-and-coding-with-kent) | plan.md + агент удаляет failing tests |
| S6 | [The Vibe-Eval Loop — LangWatch](https://langwatch.ai/scenario/best-practices/the-vibe-eval-loop/) | Итеративное дополнение спецификаций при нахождении багов |
| S7 | [ClaytonFarr/ralph-playbook](https://github.com/ClaytonFarr/ralph-playbook) | Enhancement 2, Guardrail 999, Topic Scope Test |
