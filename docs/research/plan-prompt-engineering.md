# Исследование: Структура промпта plan.md для генерации качественных планов

**Дата:** 2026-03-08
**Контекст:** Go CLI "ralph" — автономная разработка. LLM-промпт plan.md генерирует sprint-tasks.md.

---

## Executive Summary

- Contrastive prompting (WRONG/CORRECT примеры) увеличивает точность соблюдения правил на 7–53% в зависимости от задачи — это **самый эффективный инструмент** для промптов типа plan.md [S1, S9].
- Ограничения (constraints) должны стоять **до** инструкций и примеров, а не после — позиционный bias LLM делает первые constraints более соблюдаемыми [S7].
- Текущий plan.md имеет правильные WRONG/CORRECT примеры, но они неполны: нет примеров проблем зависимостей и cascade failures.
- Atomic task specification требует явного объявления зависимостей как inline-контракта — именно этот паттерн используют лучшие AI-агенты (Bolt, Cline, OpenDevin) [S5].
- Chain-of-Thought инструкция "сначала проанализируй AC, затем сгруппируй, затем упорядочи" значительно улучшает качество планов по сравнению с прямой инструкцией "генерируй задачи" [S2].

---

## Анализ текущего plan.md

### Сильные стороны

1. **WRONG/CORRECT примеры присутствуют** — правильный подход, поддержанный литературой.
2. **Таблица anti-patterns** — структурированный способ передачи ограничений.
3. **Конкретные правила** (1–3 файла, ~150 строк) — числовые constraints лучше соблюдаются чем абстрактные [S6].
4. **Инструкция про зависимости** (inline-контракт) — правильный подход из spec-driven development [S4].

### Слабые стороны (задокументированные проблемы)

1. **Нет CoT-шага анализа** — промпт сразу говорит "генерируй задачи", минуя шаг разбора.
2. **Constraints стоят после примеров** — section "Task Anti-Patterns" идёт последней, а должна быть ближе к началу.
3. **WRONG/CORRECT пример только один** — показывает bundling, но не показывает:
   - Как НЕПРАВИЛЬНО описывать зависимости (через контекст предыдущей задачи)
   - Как НЕПРАВИЛЬНО выносить тесты в отдельную задачу без связи
   - Cascade failure сценарий
4. **Инструкция про зависимости** слишком короткая и стоит в середине списка (пункт 4 из 6) — LLM теряет её.
5. **Нет примера правильного inline dependency declaration** — только текстовое описание.

---

## Ключевые находки из литературы

### 1. Техники prompt engineering для декомпозиции кода

**Chain-of-Thought + Few-Shot (наиболее эффективная комбинация):**
Комбинация CoT с few-shot примерами особенно эффективна для сложных задач планирования [S2]. Plan-and-Solve prompting явно структурирует разбор задачи через фазы: анализ → планирование → исполнение.

**Atom-of-Thoughts (AoT) — 2025:**
Разбивает задачу на самодостаточные атомарные шаги, которые можно решать параллельно или последовательно. Ключевое отличие от обычного CoT: каждый шаг явно самодостаточен [S8].

**Практическое применение для plan.md:**
Нужно добавить явный CoT-шаг ПЕРЕД генерацией задач:
```
Шаг 1: Перечисли все AC из входных документов пронумерованным списком.
Шаг 2: Сгруппируй AC по затрагиваемым файлам и слоям архитектуры.
Шаг 3: Определи зависимости между группами.
Шаг 4: Генерируй задачи в порядке зависимостей.
```

### 2. Эффективность WRONG/CORRECT примеров

**Contrastive Chain-of-Thought (CCot):**
Добавление "Let's give a correct and a wrong answer" к zero-shot промпту увеличило accuracy на GSM8K с 35.9% до 88.8% (GPT-4) [S9]. Метод работает без hand-crafted примеров.

**LEAP (Learning from mIstakes):**
Метод контрастного обучения через ошибочные и правильные chains-of-thought улучшает над стандартным few-shot на 7.5% в DROP и 3.3% в HotpotQA [S9].

**Learning from Contrastive Prompts (LCP):**
89% win rate над существующими методами при использовании анализа паттернов в хороших и плохих примерах [S9].

**Практические правила для контрастных примеров:**

| Принцип | Плохо | Хорошо |
|---|---|---|
| Конкретность | "Плохой: слишком широкий" | Показать КОНКРЕТНО что не так ("and" между слоями) |
| Размер | Один пример на все anti-patterns | Отдельный пример под каждый тип ошибки |
| Аннотация | Просто показать WRONG/CORRECT | Объяснить ПОЧЕМУ WRONG — какое правило нарушено |
| Количество | 1 пример | 3–4 примера для разных классов ошибок |

**Критическая проблема текущего промпта:**
Текущий WRONG/CORRECT пример показывает только bundling. Он НЕ показывает:
- Задачу без inline dependency declaration (cascade failure)
- Задачу с тестами в отдельном файле без контракта
- Вагую задачу ("implement feature") без конкретных имён файлов

### 3. Написание constraints которые LLM соблюдает

**Числовые constraints vs абстрактные:**
"no more than three sentences" соблюдается значительно лучше чем "be concise" [S6]. Текущие правила (1–3 файла, ~150 строк) — правильный подход.

**Позиция constraints в промпте:**
Исследование 18 LLM-моделей показало: критические ограничения должны быть в system prompt, а не user prompt. "The top instruction wins in system-user collisions" [S7].

**Структура раздела constraints (из Bolt/Cline практик) [S5]:**
```
constraints → tools/actions → planning guidance → examples
```
Т.е. ограничения ПЕРЕД инструкциями и примерами, не после.

**Multi-constraint compliance:**
При множестве ограничений LLM игнорирует часть из них, особенно последние [S7]. Решение — группировать constraints по категориям и повторять критические.

**Claude-специфика:**
"Claude tends to over-explain unless boundaries are clearly defined" — для Claude особенно важно давать explicit output format constraints [S7].

### 4. Оптимальная структура промпта

**Эмпирически установленный порядок** из практики Bolt, Cline, GitHub Copilot [S5, S10]:

```
1. Role/Persona (краткое)
2. Critical Constraints (ПЕРЕД всем остальным)
3. Task Definition
4. CoT Instructions (step-by-step reasoning process)
5. Examples (WRONG then CORRECT, с аннотациями)
6. Anti-Pattern Table
7. Output Format
```

**Vs текущий план.md:**
```
1. Role
2. Input Documents (context)
3. Task Format + примеры [ПРАВИЛА ЗДЕСЬ]
4. Instructions (6 шагов)
5. Anti-Patterns Table
```

Проблема: constraints и примеры перемешаны с форматом, CoT нет, anti-patterns идут последними.

**Instruction placement research:**
"Put instructions at the beginning or end of a prompt" — не в середине. Пункт 4 из 6 инструкций (про зависимости) — потенциально слабое место [S3].

### 5. Как другие системы формулируют задачи для AI-агентов

**Spec-Driven Development (Augment Code) [S4]:**
Четыре фазы: Specify → Plan → Tasks → Implement.
Задача содержит: objective + user journeys + success criteria + constraints.
Зависимости управляются через MCP-серверы (programmatic), не через текст промпта.

**Bolt (WebContainer) [S5]:**
- "The order of actions is VERY IMPORTANT" — зависимости объявляются ЯВНО как ordered list
- Constraints идут перед описанием инструментов
- "Think HOLISTICALLY and COMPREHENSIVELY BEFORE creating" — CoT-шаг обязателен

**Cline [S5]:**
- Один инструмент за одно сообщение (atomic)
- Explicit XML format для каждого action
- WRONG format → CORRECT format с explicit label "Always adhere to this format"

**GitHub Copilot / GitHub Blog [S10]:**
- `.spec.md` файлы как implementation-ready blueprints
- Validation gates как explicit checkpoints
- Memory-driven development через `.memory.md` — проектный контекст между сессиями

**OpenDevin/OpenHands [S8]:**
- Operates at repository level
- Multi-file changes в sandboxed environments
- Iterative debugging до работающего результата

**Ключевой вывод о зависимостях:**
Ни одна из систем не описывает зависимости через "assumes X exists" в тексте задачи. Они либо используют programmatic ordering (Spec-Driven), либо explicit numbered list (Bolt), либо sequential conversation (Cline). Текущий inline-контракт в plan.md ("assumes `RunnerOpts.PlanMode string` field exists") — это правильный подход для текстового промпта, но он должен быть более заметен через шаблон.

---

## Рекомендации по переработке plan.md

### Приоритет 1: Добавить CoT-шаг анализа (КРИТИЧНО)

Вместо "Read ALL input documents carefully" добавить явный reasoning process:

```markdown
## Pre-Generation Analysis (REQUIRED — do this before writing any tasks)

Before generating tasks, complete this analysis in your response:

**Step 1 — AC Inventory:** List all acceptance criteria found in input documents, numbered.
**Step 2 — File Grouping:** Group ACs by which files/packages they touch.
**Step 3 — Dependency Order:** Identify which groups must complete before others can start.
**Step 4 — Size Check:** Flag any group that would produce >150 lines — split it.

Then generate tasks following the analysis.
```

**Обоснование:** Plan-and-Solve и CoT prompting показывают значительное улучшение для задач планирования по сравнению с прямой инструкцией [S2]. Явный reasoning step перед output — стандарт лучших систем (Bolt: "Think HOLISTICALLY BEFORE creating").

### Приоритет 2: Переместить Constraints наверх (ВАЖНО)

Текущий порядок: Role → Input Format → Task Format → Instructions → Anti-Patterns
Новый порядок: Role → Input Format → **Critical Rules** → Task Format → Instructions → Anti-Patterns → Examples

```markdown
## Critical Rules (read before anything else)

1. **1–3 files per task maximum** — if a task touches more, split it
2. **Unit tests stay WITH implementation** in the same task — never split test from code it tests
3. **Each task is memory-isolated** — agent has NO context from previous tasks; name all interfaces explicitly
4. **No cascade dependencies** — task B must not require reading task A's output; declare the contract inline
```

**Обоснование:** "Constraints before tools/actions before examples" — эмпирически установленный порядок [S5]. "The top instruction wins" [S7]. Позиционный bias LLM делает первые правила более соблюдаемыми.

### Приоритет 3: Расширить контрастные примеры (ВАЖНО)

Добавить 3 дополнительных WRONG/CORRECT пары для каждого класса ошибок:

**Пример 2: Cascade failure через неявную зависимость**
```
WRONG — cascade failure (task B needs task A's context):
- [ ] Add `PlanMode` field to `config.Config`
  source: story.md#AC-1
- [ ] Wire `--plan-mode` CLI flag
  source: story.md#AC-2
```
*(Task 2 fails if task 1 wasn't done — agent doesn't know about PlanMode)*

```
CORRECT — inline contract (task B is self-contained):
- [ ] Wire `--plan-mode` CLI flag in `cmd/ralph/plan.go`; assumes `cfg.PlanMode string` field
  already exists in `config/config.go` (added by previous task)
  source: story.md#AC-2
```

**Пример 3: Тесты без контракта**
```
WRONG — тест в отдельной задаче без контракта:
- [ ] Add `applyPlanDefaults()` to config package
  source: story.md#AC-1
- [ ] Write tests for config defaults
  source: story.md#AC-1
```
*(Agent writing tests doesn't know the function signature)*

```
CORRECT — unit test вместе с реализацией:
- [ ] Add `applyPlanDefaults()` to `config/config.go` called from `Load()`;
  write `TestApplyPlanDefaults_Defaults` and `TestApplyPlanDefaults_Override` in `config/config_test.go`
  source: story.md#AC-1
```

**Обоснование:** LCP framework с контрастными примерами дает 89% win rate [S9]. Каждый класс ошибок нуждается в отдельном примере — один пример не обобщается на другие типы bundling.

### Приоритет 4: Усилить описание зависимостей (ВАЖНО)

Текущая формулировка (пункт 4):
> "If task B depends on task A, describe the expected interface/signature from A inline as an external contract"

Нужно сделать это обязательным шаблоном:

```markdown
**Dependency Declaration Template** (use when task B requires output from task A):
```
- [ ] [Task B description]; **contract from prior tasks**: `FunctionName(param Type) ReturnType`
  defined in `package/file.go`
  source: story.md#AC-N
```
```

**Обоснование:** Explicit ordering и contract declaration — стандарт всех успешных AI-агентных систем [S4, S5, S10]. Без шаблона агенты не следуют текстовым инструкциям.

### Приоритет 5: Аннотировать WRONG примеры (УМЕРЕННО)

К каждому WRONG примеру добавить объяснение, какое правило нарушено:

```
WRONG — bundle task (violates Rule 1: spans 3 files + 3 ACs):
- [ ] Implement config loading and add tests and wire CLI flag
```

**Обоснование:** LEAP метод показывает, что объяснение ПОЧЕМУ пример плохой важнее чем просто показать плохой пример [S9].

---

## Что убрать / не менять

| Элемент | Решение | Обоснование |
|---|---|---|
| Числовые constraints (1–3 файла, ~150 строк) | СОХРАНИТЬ | Числовые limits соблюдаются лучше абстрактных [S6] |
| Таблица Anti-Patterns | СОХРАНИТЬ, переместить выше | Структурированный формат эффективен |
| WRONG/CORRECT пример bundling | СОХРАНИТЬ, добавить аннотацию | Уже правильный подход |
| `[GATE]` / `[SETUP]` теги | СОХРАНИТЬ | Правильные семантические маркеры |
| Replan/Merge mode блоки | СОХРАНИТЬ | Функциональная необходимость |
| Инструкция "name specific files" | СОХРАНИТЬ, усилить шаблоном | Правильный принцип для isolation |

---

## Итоговая рекомендованная структура plan.md

```
1. Role (1-2 строки)
2. Output Constraint (IMPORTANT: output only raw content)
3. Input Documents section
4. Critical Rules (4 numbered rules — ПЕРЕД примерами)
5. Dependency Declaration Template (шаблон с примером)
6. Task Format (формат строки задачи)
7. Examples:
   - Example 1: Bundling (WRONG → CORRECT) с аннотацией
   - Example 2: Cascade failure (WRONG → CORRECT) с аннотацией
   - Example 3: Tests separated (WRONG → CORRECT) с аннотацией
8. Pre-Generation Analysis (CoT шаг — REQUIRED)
9. Instructions (сокращённые, т.к. CoT уже выше)
10. Anti-Patterns Table
11. Replan/Merge блоки (без изменений)
```

---

## Evidence Table

| ID | Источник | Релевантность | Качество |
|---|---|---|---|
| S1 | ScienceDirect: LLMs are Contrastive Reasoners | Contrastive prompting +52% accuracy | A |
| S2 | PromptingGuide: CoT Prompting | CoT for task decomposition | A |
| S3 | LearnPrompting: Prompt Structure | Ordering of prompt sections | B |
| S4 | Augment Code: Spec-Driven Development | Atomic task format for AI agents | B |
| S5 | PromptHub: AI Agent Prompts | Bolt/Cline constraint ordering patterns | B |
| S6 | Palantir: Prompt Best Practices | Numeric constraints compliance | B |
| S7 | AIMuse: System vs User Prompts, 18-model benchmark | System prompt placement, constraint ordering | A |
| S8 | AICerts: Atom-of-Thoughts | Atomic self-contained steps | B |
| S9 | LearnPrompting: Contrastive CoT | WRONG/CORRECT examples effectiveness | A |
| S10 | GitHub Blog: Agentic Primitives | Spec-driven, dependency handling | B |

---

## Источники

- [Large language models are contrastive reasoners](https://www.sciencedirect.com/science/article/abs/pii/S0957417425040229)
- [Chain-of-Thought Prompting | Prompt Engineering Guide](https://www.promptingguide.ai/techniques/cot)
- [Contrastive Chain-of-Thought Prompting | LearnPrompting](https://learnprompting.org/docs/advanced/thought_generation/contrastive_cot)
- [AI Coding Agents for Spec-Driven Development | Augment Code](https://www.augmentcode.com/guides/ai-coding-agents-for-spec-driven-development-automation)
- [Prompt Engineering for AI Agents | PromptHub](https://www.prompthub.us/blog/prompt-engineering-for-ai-agents)
- [Best practices for LLM prompt engineering | Palantir](https://www.palantir.com/docs/foundry/aip/best-practices-prompt-engineering)
- [System Prompts vs User Prompts | AIMuse (18-model benchmark)](https://aimuse.blog/article/2025/06/14/system-prompts-versus-user-prompts-empirical-lessons-from-an-18-model-llm-benchmark-on-hard-constraints)
- [Atom-of-Thoughts | AICerts](https://www.aicerts.ai/blog/atom-of-thoughts-the-prompt-engineering-breakthrough-you-need-to-know/)
- [How to build reliable AI workflows | GitHub Blog](https://github.blog/ai-and-ml/github-copilot/how-to-build-reliable-ai-workflows-with-agentic-primitives-and-context-engineering/)
- [Generating reliable software project task flows using LLMs | Nature](https://www.nature.com/articles/s41598-025-19170-9)
- [In-Context Principle Learning from Mistakes (LEAP)](https://arxiv.org/html/2402.05403v2)
- [Few-Shot Prompting | Prompt Engineering Guide](https://www.promptingguide.ai/techniques/fewshot)
