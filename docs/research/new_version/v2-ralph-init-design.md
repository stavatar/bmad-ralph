# ralph init — Дизайн команды быстрого старта

**Дата:** 2026-03-07
**Контекст:** Сейчас время от идеи до первого коммита ralph — 55-130 минут (BMad pipeline). Конкуренты: 1-5 минут. `ralph init` сокращает до 2-5 минут.

---

## 1. Проблема

Ralph сейчас требует:
1. Установить BMad Method
2. Пройти 4 workflow (PRD, Architecture, Epics, Stories) — 45-100 мин
3. Запустить `ralph bridge` — 10-30 мин
4. Получить `sprint-tasks.md`
5. `ralph run`

Это барьер для adoption. Человек, который хочет попробовать ralph на новом проекте, уходит к Cursor/Claude Code/Aider, потому что там можно начать через 1-3 минуты.

## 2. Позиционирование ralph на спектре инструментов

```
Минимум структуры                                    Максимум структуры
     |                                                        |
  Cursor    Aider    Claude Code    Devin    ralph init    ralph+BMad
  "делай"   "делай"  "CLAUDE.md"   "Issue"  "требования"  "PRD→Stories"
  1 мин     1 мин    1-3 мин       2-5 мин  2-5 мин       55-130 мин
```

**Ниша ralph** — между Devin и полным BMad:
- Больше структуры, чем "просто делай" (Cursor/Aider) — ralph даёт review, gates, knowledge extraction
- Меньше церемонии, чем BMad — не нужны PRD, Architecture, Epics, Stories
- Уникальная ценность: автономный multi-session loop с quality gates, а не one-shot execution

**Ключевой инсайт:** ralph init не конкурирует с "напиши функцию" (это территория Cursor/Aider). ralph init для проектов, где нужно 10-100+ задач с code review и quality control.

## 3. Три flow команды ralph init

### 3.1. Минимальный flow (one-liner)

```bash
ralph init "Платформа для обучения Java с sandbox, JWT auth, Monaco editor"
```

**Что происходит:**
1. ralph создаёт `docs/requirements.md` — вызывает Claude с промптом "структурируй требования"
2. Claude получает описание + результат сканирования проекта (если есть файлы)
3. Генерирует компактный документ (см. секцию 4 — формат)
4. ralph выводит: "Создан docs/requirements.md. Проверьте и отредактируйте, затем: ralph plan"

**Стоимость:** ~$0.10-0.30 (один LLM-вызов, ~2000 токенов output)
**Время:** 30-90 секунд

### 3.2. Интерактивный flow

```bash
ralph init --interactive
```

**Что происходит:**
1. ralph задаёт 5-7 вопросов через stdin (НЕ через LLM — фиксированные вопросы):

```
Проект: ___
Опиши проект в 1-3 предложениях: ___
Tech stack (языки, фреймворки): ___
Основные фичи (через запятую): ___
Есть ли внешние зависимости (DB, API, очереди)? ___
Масштаб (сколько пользователей, сколько данных)? ___
Есть ли особые требования (безопасность, performance, compliance)? ___
```

2. ralph собирает ответы в structured prompt
3. Вызывает Claude — генерирует `docs/prd.md` + `docs/architecture.md` (раздельно)
4. Выводит: "Созданы docs/prd.md и docs/architecture.md. Проверьте, затем: ralph plan"

**Стоимость:** ~$0.30-0.80 (один LLM-вызов, ~4000-6000 токенов output)
**Время:** 2-5 минут (включая ввод ответов)

### 3.3. Brownfield flow (существующий проект)

```bash
ralph init --scan
```

**Что происходит:**
1. ralph сканирует проект программно (без LLM):
   - `go.mod` / `package.json` / `Cargo.toml` / `pyproject.toml` / `pom.xml` — стек
   - `Dockerfile` / `docker-compose.yml` — инфраструктура
   - `README.md` — описание (первые 50 строк)
   - `.env.example` — переменные окружения
   - Структура директорий (глубина 2)
   - `CLAUDE.md` / `.cursor/` — существующие AI-конфиги

2. Вызывает Claude с результатом сканирования:
   - "Проанализируй проект и создай docs/project-context.md"

3. Генерирует `docs/project-context.md` с секциями:
   - Стек и зависимости
   - Архитектура (на основе структуры)
   - Текущее состояние
   - Зона для требований (пустая — пользователь заполняет)

4. Выводит: "Создан docs/project-context.md. Добавьте требования в секцию 'Требования', затем: ralph plan"

**Стоимость:** ~$0.10-0.30
**Время:** 30-90 секунд

## 4. Формат requirements.md

### 4.1. Минимальная структура

```markdown
# Требования проекта

## Описание
Платформа для обучения Java с sandbox, JWT auth, Monaco editor.

## Стек
- Backend: Java 21, Spring Boot 3, PostgreSQL
- Frontend: React 19, TypeScript, Vite
- Инфраструктура: Docker (sandbox для Java-кода)

## Функциональные требования
- FR1: Вход по инвайт-ссылке с минимальной идентификацией
- FR2: Каталог задач с фильтрацией по теме и сложности
- FR3: Редактор кода с подсветкой синтаксиса (Monaco)
- FR4: Автопроверка решений через JUnit-тесты в sandbox
- FR5: Прогресс ученика по темам

## Нефункциональные требования
- NFR1: Sandbox: Docker, таймаут 30с, RAM 256MB
- NFR2: До 100 одновременных пользователей

## Ограничения
- Закрытая платформа, доступ по инвайту
- Один ментор, одна группа
```

### 4.2. Чем отличается от полного BMad PRD

| Аспект | requirements.md | BMad PRD |
|--------|----------------|----------|
| **Объём** | 30-80 строк | 200-500 строк |
| **User Journeys** | Нет | 3-5 детальных |
| **Success Criteria** | Нет (или 2-3 строки) | Секция с KPI |
| **Growth/Vision** | Нет | Да |
| **FR нумерация** | Свободная (FR1-FRN) | Структурированная по группам |
| **NFR** | 2-5 пунктов | 10-15 с таблицей |
| **Scope** | MVP only | MVP + Growth + Vision |
| **UI/UX** | Нет | Отдельный документ |

**Ключевое различие:** requirements.md — это "достаточно для начала работы". BMad PRD — это "полная спецификация для передачи другой команде". Для ralph (который сам будет реализовывать) достаточно requirements.md.

### 4.3. Может ли быть plain text?

**Да, но с ограничениями.** ralph plan должен уметь принимать:

```
Сделай платформу для обучения Java. Нужен редактор кода,
автопроверка через тесты, каталог задач по темам.
Стек: Spring Boot, React, Docker для sandbox.
```

Это работает для простых проектов (5-15 задач). Для сложных (50+ задач) недостаточно контекста — Claude будет додумывать.

**Рекомендация:** plain text принимается, но ralph plan предупреждает:
```
Обнаружен plain text без структуры. Для проектов >15 задач рекомендуется
ralph init для создания структурированного requirements.md.
Продолжить? [y/N]
```

## 5. Команда ralph plan

### 5.1. Что делает

```bash
ralph plan [docs/requirements.md]
```

1. Читает requirements.md (или auto-detect: `docs/requirements.md`, `docs/prd.md`, `requirements.md`)
2. Вызывает Claude с промптом декомпозиции
3. Генерирует `sprint-tasks.yaml` (YAML, не markdown — см. variant-d-task-format.md)
4. Выводит summary: "Создано N задач в sprint-tasks.yaml"

### 5.2. Промпт декомпозиции

Промпт для Claude содержит:
- Требования из файла
- Результат сканирования проекта (если brownfield)
- Инструкции по декомпозиции:
  - Атомарные задачи (одна задача = один чёткий результат)
  - Зависимости между задачами
  - Приоритеты (foundation → features → polish)
  - Оценка сложности (S/M/L)

### 5.3. Формат sprint-tasks.yaml

```yaml
project: "learnPracticsCodePlatform"
generated: "2026-03-07T10:00:00Z"
source: "docs/requirements.md"

tasks:
  - id: "setup-monorepo"
    title: "Инициализировать monorepo: Turborepo + apps/client + apps/server + packages/shared"
    type: scaffold
    size: M
    depends_on: []
    requirements: [FR1, FR2]
    status: open

  - id: "auth-jwt"
    title: "JWT аутентификация: инвайт-ссылка → токен → guard middleware"
    type: feature
    size: M
    depends_on: [setup-monorepo]
    requirements: [FR1, NFR1]
    status: open

  - id: "sandbox-docker"
    title: "Docker sandbox: warm pool, 30с таймаут, 256MB RAM лимит"
    type: feature
    size: L
    depends_on: [setup-monorepo]
    requirements: [FR4, NFR1, NFR2]
    status: open
```

**Преимущества YAML над markdown:**
- Машиночитаемость (yaml.v3 уже в зависимостях ralph)
- Зависимости (`depends_on`) — нативно
- Нет regex-парсинга — нет проблем с `- []` vs `- [ ]`
- Schema validation через Go struct

### 5.4. Инкрементальный plan

```bash
# Первый запуск
ralph plan docs/requirements.md
# Создаёт sprint-tasks.yaml

# После изменения requirements
ralph plan docs/requirements.md --update
# Добавляет новые задачи, не трогает completed
```

## 6. Полный pipeline

### 6.1. Greenfield (новый проект) — минимальный

```bash
mkdir my-project && cd my-project
git init

ralph init "REST API для управления задачами на Go с PostgreSQL и JWT"
# → docs/requirements.md (30 сек)

# Пользователь проверяет/правит requirements.md (1-2 мин)

ralph plan
# → sprint-tasks.yaml (30 сек)

ralph run
# → Начинает выполнение задач
```

**Общее время: 2-5 минут.**

### 6.2. Greenfield — интерактивный

```bash
ralph init --interactive
# → 5-7 вопросов (2 мин)
# → docs/prd.md + docs/architecture.md (1 мин)

ralph plan
# → sprint-tasks.yaml

ralph run
```

**Общее время: 4-7 минут.**

### 6.3. Brownfield (существующий проект)

```bash
cd existing-project

ralph init --scan
# → docs/project-context.md (30 сек)

# Пользователь добавляет требования в project-context.md (2-3 мин)

ralph plan docs/project-context.md
# → sprint-tasks.yaml

ralph run
```

### 6.4. BMad-совместимый (backward compatibility)

```bash
# Существующий BMad-проект со stories
ralph plan --stories docs/sprint-artifacts/
# → Программный парсинг story файлов (без LLM!)
# → sprint-tasks.yaml

ralph run
```

### 6.5. Прямой запуск (без init/plan)

```bash
ralph run "Добавь JWT авторизацию с refresh-токенами"
# → ralph plan запускается автоматически (in-session)
# → Создаёт sprint-tasks.yaml
# → Начинает выполнение
```

Это аналог Cursor/Aider — для маленьких задач (1-5 шагов).

## 7. Архитектура реализации

### 7.1. Новые пакеты

```
cmd/ralph/
  init.go        # CLI команда ralph init
  plan.go        # CLI команда ralph plan

planner/
  planner.go     # Оркестратор: scan → prompt → parse → write
  scanner.go     # Сканирование проекта (go.mod, package.json, etc.)
  questions.go   # Интерактивные вопросы (--interactive)
  prompt.go      # Промпт для Claude (декомпозиция)
  tasks.go       # Модель sprint-tasks.yaml (struct + marshal/unmarshal)
```

### 7.2. Интеграция с существующим кодом

```
cmd/ralph/init.go
  → planner.Scanner.Scan()          # Сканирование проекта
  → session.RunClaude()              # Генерация requirements.md
  → os.WriteFile("docs/requirements.md")

cmd/ralph/plan.go
  → planner.LoadRequirements()       # Чтение requirements
  → planner.Scanner.Scan()           # Контекст проекта
  → session.RunClaude()              # Декомпозиция в задачи
  → planner.WriteTasks()             # sprint-tasks.yaml

  ИЛИ (BMad mode):
  → planner.ParseStories()           # Программный парсинг stories
  → planner.WriteTasks()             # sprint-tasks.yaml (без LLM!)
```

### 7.3. Изменения в Config

```go
type Config struct {
    // ... существующие поля ...

    // Init/Plan
    RequirementsFile string `yaml:"requirements_file"` // default: "docs/requirements.md"
    TasksFile        string `yaml:"tasks_file"`        // default: "sprint-tasks.yaml"
    ModelPlan        string `yaml:"model_plan"`        // модель для plan/init
}
```

### 7.4. Миграция runner

Runner сейчас работает с `sprint-tasks.md` (markdown). Нужна миграция на `sprint-tasks.yaml`:

1. **Фаза 1:** runner умеет читать оба формата (auto-detect по расширению)
2. **Фаза 2:** bridge генерирует YAML вместо markdown (backward compat)
3. **Фаза 3:** bridge deprecated, markdown поддержка deprecated

### 7.5. Зависимости пакетов

```
cmd/ralph → planner, runner, session, config
planner → session, config              (leaf-ish, вызывает Claude через session)
runner → session, gates, config        (без изменений)
session → config                       (без изменений)
```

Planner НЕ зависит от runner (разделение planning и execution).

## 8. Промпты

### 8.1. Промпт для ralph init (генерация requirements.md)

```
Ты — архитектор программных систем. На основе описания проекта создай
структурированный документ требований.

Входные данные:
- Описание проекта: {user_description}
- Результат сканирования (если есть): {scan_result}

Создай документ со следующими секциями:
1. Описание — 2-3 предложения, суть проекта
2. Стек — языки, фреймворки, БД, инфраструктура
3. Функциональные требования — пронумерованный список FR1-FRN
4. Нефункциональные требования — NFR1-NFRN (если есть)
5. Ограничения — известные ограничения

Правила:
- Каждое FR — одно конкретное поведение, тестируемое
- Не додумывай то, чего нет в описании
- Лучше 10 точных FR, чем 50 размытых
- Если стек не указан — предложи обоснованный выбор
```

### 8.2. Промпт для ralph plan (декомпозиция)

```
Ты — tech lead, планирующий разработку. На основе требований создай
список атомарных задач для автономного AI-агента (Claude Code).

Входные данные:
- Требования: {requirements_content}
- Проект (если brownfield): {scan_result}

Создай задачи в YAML формате:

tasks:
  - id: "kebab-case-id"
    title: "Что конкретно сделать"
    type: scaffold|feature|refactor|bugfix|test|docs
    size: S|M|L
    depends_on: [id-зависимости]
    requirements: [FR1, FR2]

Правила декомпозиции:
- Одна задача = один контекст Claude-сессии (5-30 минут работы)
- Задачи size S: 1-3 файла, <100 строк изменений
- Задачи size M: 3-7 файлов, <300 строк
- Задачи size L: 7+ файлов, <500 строк (разбей на 2-3 M если больше)
- Foundation задачи (scaffold, config) — ПЕРЕД feature задачами
- Тесты в КАЖДОЙ feature задаче (не отдельной задачей)
- Зависимости: только прямые (A→B, не A→B→C)
- Не создавай задачи для "настройки CI" или "написания README"
  если это не указано в требованиях
```

## 9. Сравнение с конкурентами

### 9.1. Что ralph init берёт у каждого

| Конкурент | Что берём | Что НЕ берём |
|-----------|-----------|--------------|
| **Devin** | Natural language input, one-liner старт | SaaS-модель, нет local execution |
| **Cursor** | Мгновенный старт, zero config | One-shot без multi-session loop |
| **Aider** | CLI-first, plain text instructions | Нет persistent task tracking |
| **Claude Code** | CLAUDE.md как project context | Нет автономного multi-task loop |
| **SWE-Agent** | GitHub Issue как input | Только bugfix, не greenfield |
| **Copilot Workspace** | Spec → Plan → Code pipeline | Закрытый SaaS |

### 9.2. Уникальные преимущества ralph после init

1. **Multi-session loop** — 10-100 задач автономно (конкуренты: 1 задача за раз)
2. **Code review встроенный** — каждые N задач (конкуренты: нет или manual)
3. **Human gates** — контрольные точки без потери автономности
4. **Knowledge extraction** — обучение на ошибках (конкуренты: нет)
5. **Stuck detection** — не зацикливается бесконечно
6. **Budget control** — не потратит $100 на одну задачу

### 9.3. Позиционирование

```
Scope задачи:   одна функция  ←→  весь проект
                     |                  |
                  Cursor/Aider    ralph init + ralph run
                  Claude Code     BMad + ralph (для тех кто хочет)

Автономность:   one-shot  ←→  multi-session
                    |              |
                 Cursor         ralph run (10-100 задач)
                 Aider          Devin (но SaaS)
                 Claude Code
```

**ralph init** позволяет начать за 2-5 минут, но потом ralph даёт то, чего нет у конкурентов: автономный multi-session loop с quality gates.

## 10. Оценка реализации

### 10.1. Объём работ

| Компонент | LOC (оценка) | Сложность | Эпик |
|-----------|--------------|-----------|------|
| `cmd/ralph/init.go` | 80-120 | Низкая | 11 |
| `cmd/ralph/plan.go` | 100-150 | Низкая | 11 |
| `planner/scanner.go` | 150-250 | Средняя | 11 |
| `planner/questions.go` | 80-120 | Низкая | 11 |
| `planner/tasks.go` | 100-150 | Средняя | 11 |
| `planner/prompt.go` | 50-80 | Низкая | 11 |
| `planner/planner.go` | 100-150 | Средняя | 11 |
| Промпты (init + plan) | 60-100 строк | Средняя | 11 |
| Миграция runner на YAML | 200-300 | Средняя | 11 |
| Тесты | 500-800 | Средняя | 11 |
| **ИТОГО** | **1400-2200** | | |

### 10.2. Зависимости

- **Новых внешних зависимостей: 0** (yaml.v3 уже есть, cobra уже есть)
- Внутренние: `session.RunClaude()` уже существует

### 10.3. Риски

| Риск | Вероятность | Mitigation |
|------|-------------|------------|
| Claude генерирует плохие requirements | Средняя | Промпт с примерами + human review |
| Claude генерирует слишком много/мало задач | Средняя | Калибровка промпта + `--max-tasks N` |
| YAML-парсинг задач конфликтует с существующим runner | Низкая | Auto-detect формата, постепенная миграция |
| Пользователи пропускают review requirements | Высокая | Warning: "Проверьте перед plan" + `--yes` для skip |

## 11. Поэтапный план реализации

### Фаза 1: ralph plan (без init)
- Миграция runner на YAML tasks
- `ralph plan <файл>` — LLM-декомпозиция requirements в sprint-tasks.yaml
- `ralph plan --stories <dir>` — программный парсинг BMad stories
- bridge остаётся, но deprecated

### Фаза 2: ralph init
- `ralph init "описание"` — генерация requirements.md
- `ralph init --scan` — сканирование проекта
- `ralph init --interactive` — вопросы + генерация prd/architecture

### Фаза 3: ralph run "описание" (direct execution)
- `ralph run "задача"` — автоматический plan + run
- Для задач <5 шагов — не создаёт файл, планирует in-session

### Фаза 4: Deprecation bridge
- Warning при `ralph bridge`
- Удаление через 2-3 релиза

## 12. Открытые вопросы

1. **Генерация requirements.md vs ручное написание?** Для минимального flow ralph init генерирует через Claude. Но пользователь может написать сам и сразу `ralph plan`.

2. **Один файл requirements.md vs два (prd + architecture)?** Минимальный flow — один. Интерактивный — два. ralph plan принимает оба варианта.

3. **YAML vs markdown для sprint-tasks?** Рекомендация: YAML (см. variant-d-task-format.md). Но поддержка markdown сохраняется для backward compatibility.

4. **Нужен ли ralph init для brownfield?** Да, `--scan` ценен — экономит время на описание стека. Но можно обойтись ручным requirements.md.

5. **Куда класть sprint-tasks.yaml?** Варианты: корень проекта (видимость), `.ralph/tasks.yaml` (не засоряет), `docs/sprint-tasks.yaml` (рядом с requirements). Рекомендация: корень проекта — простота и прозрачность.

6. **Нужен ли `ralph init` для монорепо?** `--scan` автоматически обнаруживает workspace-структуру (turbo.json, pnpm-workspace.yaml). В requirements.md добавляется секция "Структура монорепо".
