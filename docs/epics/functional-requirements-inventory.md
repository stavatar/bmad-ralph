# Functional Requirements Inventory

### Bridge (Планирование задач)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR1** | Конвертация BMad story-файлов в структурированный sprint-tasks.md | MVP |
| **FR2** | Вывод тест-кейсов из объективных AC. Субъективные AC помечаются для ручной/LLM-as-Judge верификации (post-MVP) | MVP |
| **FR3** | Разметка точек human gate тегом `[GATE]` в sprint-tasks.md (первая задача epic'а, user-visible milestones) | MVP |
| **FR4** | Smart Merge при повторном запуске bridge с существующим sprint-tasks.md | MVP |
| **FR5** | Генерация служебных задач: project setup, integration verification, e2e checkpoint (Growth) | MVP |
| **FR5a** | Поле `source:` в каждой задаче — трассировка задача → story + AC | MVP |

### Execute (Автономное выполнение)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR6** | Последовательное выполнение задач из sprint-tasks.md. Git health check при старте (clean state, не detached HEAD) | MVP |
| **FR7** | Каждое выполнение задачи — в свежей сессии Claude Code | MVP |
| **FR8** | Execute: читает задачу, реализует код, запускает unit-тесты, коммитит при green. e2e только для UI-задач и checkpoint-ов | MVP |
| **FR9** | Retry до max iterations. Успешность по наличию git коммита. Resume-extraction при failure. Два счётчика: `execute_attempts` и `review_cycles` | MVP |
| **FR10** | Настраиваемый max turns per execute session (`--max-turns`) | MVP |
| **FR11** | Claude self-directing: читает sprint-tasks.md, берёт первую `- [ ]`. Review ставит `[x]`. Execute НЕ меняет статус задач. Ralph сканирует только для loop control | MVP |
| **FR12** | Продолжение с первой незавершённой при re-run. Dirty tree → `git checkout -- .`. Мягкая валидация формата | MVP |

### Review (Ревью кода)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR13** | Review после каждой выполненной задачи | MVP |
| **FR14** | Review в отдельной свежей сессии Claude Code | MVP |
| **FR15** | 4 параллельных sub-агента через Task tool: quality, implementation, simplification, test-coverage | MVP |
| **FR16** | Верификация findings sub-агентов: CONFIRMED / FALSE POSITIVE. Severity: CRITICAL/HIGH/MEDIUM/LOW | MVP |
| **FR16a** | Severity filtering: findings ниже порога (`review_min_severity`) в лог, не блокируют pipeline | Growth |
| **FR17** | Review ТОЛЬКО анализирует. Clean → `[x]` + clear findings. Findings → overwrite review-findings.md + lessons в LEARNINGS.md + CLAUDE.md | MVP |
| **FR18** | При findings → следующий execute адресует findings (единый тип execute-сессии) | MVP |
| **FR18a** | Повторный review для верификации фиксов (цикл execute→review, до `max_review_iterations`, default 3) | MVP |
| **FR19** | Batch review (`--review-every N`) с аннотированным diff и маппингом TASK→AC→тесты | Growth |

### Gates (Контроль качества)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR20** | Включение human gates через CLI-флаг `--gates` | MVP |
| **FR21** | Остановка на размеченных точках human gate | MVP |
| **FR22** | Approve, retry (feedback → fix-задача), skip, quit. Ralph добавляет feedback в sprint-tasks.md | MVP |
| **FR23** | Emergency human gate при исчерпании max execute attempts | MVP |
| **FR24** | Emergency human gate при превышении max review iterations | MVP |
| **FR25** | Периодические checkpoint gates каждые N задач (`--every N`) | MVP |

### Knowledge (Управление знаниями)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR26** | Операционные знания → секция `## Ralph operational context` в CLAUDE.md. Обновление через review и resume-extraction | MVP |
| **FR27** | Паттерны и выводы → LEARNINGS.md (append с hard limit 200 строк) | MVP |
| **FR28** | Resume-extraction при неудачном execute (`claude --resume`): WIP commit + progress в sprint-tasks.md + знания в LEARNINGS.md | MVP |
| **FR28a** | Review записывает lessons при findings (без отдельной extraction). Distillation при превышении бюджета LEARNINGS.md | MVP |
| **FR28b** | `--always-extract` — extraction знаний после каждого execute (не только failure) | MVP |
| **FR29** | Knowledge files загружаются в контекст каждой новой сессии | MVP |

### Config (Конфигурация и кастомизация)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR30** | Config файл `.ralph/config.yaml` в корне проекта (16 параметров) | MVP |
| **FR31** | CLI flags override config file override embedded defaults | MVP |
| **FR32** | Кастомизация промптов review-агентов через `.md` файлы | MVP |
| **FR33** | Fallback chain: `.ralph/agents/` (project) → `~/.config/ralph/agents/` (global) → embedded defaults | MVP |
| **FR34** | Per-agent model configuration (execute: opus, review agents: sonnet/haiku) | MVP |
| **FR35** | Информативные exit codes (0-4) для интеграции со скриптами | MVP |

### Guardrails и ATDD

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR36** | 999-series guardrail-правила в execute-промпте. Последний барьер: даже при опасном finding → execute откажется | MVP |
| **FR37** | ATDD enforcement: каждый AC покрыт тестом | MVP |
| **FR38** | Zero test skip: unit на каждый execute, e2e на UI-задачах и checkpoint-ах. Падения исправляются или эскалируются | MVP |
| **FR39** | Serena MCP detection + integration (best effort). Full index при старте, incremental перед execute, graceful fallback | MVP |
| **FR40** | CLI version check + session adapter для multi-LLM | Growth |
| **FR41** | Context budget calculator: подсчёт размера контекста перед сессией, warning при >40% context window | Growth |

### Итого

| Категория | MVP | Growth | Всего |
|-----------|:---:|:------:|:-----:|
| Bridge | 6 | 0 | 6 |
| Execute | 7 | 0 | 7 |
| Review | 7 | 2 | 9 |
| Gates | 6 | 0 | 6 |
| Knowledge | 6 | 0 | 6 |
| Config | 6 | 0 | 6 |
| Guardrails/ATDD | 4 | 2 | 6 |
| **Всего** | **42** | **4** | **46** |

> **Примечание (Party Mode):** FR28b (`--always-extract`) и FR39 (Serena) — MVP, но nice-to-have внутри MVP. Планируются в последнем эпике чтобы не блокировать core flow.

---
