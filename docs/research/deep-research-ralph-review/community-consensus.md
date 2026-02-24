# Консенсус сообщества Ralph Loop по вопросам код-ревью

**Дата исследования:** 24 февраля 2026
**Область:** Подходы к код-ревью в контексте Ralph Loop (Ralph Wiggum Technique)

---

## 1. Введение и контекст

Ralph Loop (техника Ralph Wiggum) -- методология автономной AI-разработки, созданная Geoffrey Huntley. Суть техники: бесконечный цикл, который повторно подает один и тот же промпт AI-агенту (Claude Code, Codex и др.), пока задача не будет завершена. Прогресс сохраняется не в контекстном окне LLM, а в файлах и git-истории. Название отсылает к персонажу Ральфу Виггуму из "Симпсонов" -- воплощению упорства несмотря на неудачи.

К февралю 2026 года Ralph Loop стал одной из наиболее обсуждаемых техник в AI-разработке. Многие участники Y Combinator активно применяют Ralph, Anthropic выпустила официальный Ralph Wiggum Plugin для Claude Code, а экосистема форков и реализаций насчитывает десятки проектов.

**Ключевой вопрос этого исследования:** как сообщество подходит к код-ревью в контексте Ralph Loop, и какие подходы считаются правильными.

### Источники

- [Geoffrey Huntley -- "Everything is a Ralph Loop"](https://ghuntley.com/loop/)
- [Geoffrey Huntley -- "Ralph Wiggum as a Software Engineer"](https://ghuntley.com/ralph/)
- [Inventing the Ralph Wiggum Loop | Dev Interrupted (LinearB Podcast)](https://linearb.io/dev-interrupted/podcast/inventing-the-ralph-wiggum-loop)
- [Mastering Ralph Loops | LinearB Blog](https://linearb.io/blog/ralph-loop-agentic-engineering-geoffrey-huntley)
- [2026: The Year of the Ralph Loop Agent | DEV Community](https://dev.to/alexandergekov/2026-the-year-of-the-ralph-loop-agent-1gkj)
- [Ralph Wiggum Explained | Dev Genius](https://blog.devgenius.io/ralph-wiggum-explained-the-claude-code-loop-that-keeps-going-3250dcc30809)
- [A Brief History of Ralph | HumanLayer](https://www.humanlayer.dev/blog/brief-history-of-ralph)
- [VentureBeat -- How Ralph Wiggum went from The Simpsons to AI](https://venturebeat.com/technology/how-ralph-wiggum-went-from-the-simpsons-to-the-biggest-name-in-ai-right-now)
- ['Ralph Wiggum' loop prompts Claude to vibe-clone software | The Register](https://www.theregister.com/2026/01/27/ralph_wiggum_claude_loops/)

---

## 2. Философия свежего контекста (Fresh Context Principle)

### 2.1. Суть принципа

Центральная идея Ralph Loop -- **каждая итерация начинается со свежим контекстом**. Стандартные агентные циклы страдают от накопления контекста: каждая неудачная попытка остается в истории разговора. После нескольких итераций модель вынуждена обрабатывать длинную историю "шума" прежде чем сфокусироваться на задаче.

Geoffrey Huntley формулирует это как: **"Одно контекстное окно, одна активность, одна цель"** ("One context window, one activity, one goal"). Каждая итерация запускает полностью новый процесс Claude, который завершается по окончании работы.

### 2.2. Две зоны производительности

Сообщество выделяет две зоны работы модели:

- **Smart Zone (0--100K токенов):** Claude работает на полной мощности
- **Dumb Zone (100K--200K токенов):** качество значительно деградирует, приводя к плохим решениям

Ralph Loop поддерживает когнитивную "остроту" модели через регулярный сброс контекстного окна, сохраняя накопленные знания через документацию и файловую систему.

### 2.3. Память через файловую систему

Память в Ralph Loop живет не в контексте LLM, а на диске:

- **fix_plan.md / IMPLEMENTATION_PLAN.md** -- отслеживает выполненные и ожидающие задачи
- **AGENT.md / AGENTS.md** -- документирует как собирать и запускать проект
- **progress.txt** -- краткое резюме прогресса
- **Git-коммиты** -- сохраняют реальные изменения кода

### Источники

- [Fresh Context Pattern (Ralph Loop) | DeepWiki](https://deepwiki.com/FlorianBruniaux/claude-code-ultimate-guide/7.3-fresh-context-pattern-(ralph-loop))
- [Everyone's Using Ralph Loops Wrong | Sparkry AI](https://sparkryai.substack.com/p/everyones-using-ralph-loops-wrong)
- [The Real Ralph Wiggum Loop: What Everyone Gets Wrong | TheTrav](https://thetrav.substack.com/p/the-real-ralph-wiggum-loop-what-everyone)
- [Ralph Loops: What Most Developers Get Wrong | Medium/Vibe Coding](https://medium.com/vibe-coding/everyones-using-ralph-loops-wrong-here-s-what-actually-works-e5e4208873c1)
- [From ReAct to Ralph Loop | Alibaba Cloud](https://www.alibabacloud.com/blog/from-react-to-ralph-loop-a-continuous-iteration-paradigm-for-ai-agents_602799)

---

## 3. Подходы к код-ревью: таксономия

Сообщество выработало несколько принципиально различных подходов к код-ревью в контексте Ralph Loop. Ниже представлена их систематизация.

### 3.1. Подход A: Ревью внутри итерации (Review Inside Iteration)

**Описание:** Код-ревью выполняется в той же итерации, что и реализация. Агент-работник пишет код, затем (в рамках того же контекстного окна) выполняется проверка -- тесты, линтинг, типизация.

**Представители:**
- Ralph Playbook (Clayton Farr) -- ревью реализуется как "backpressure": тесты, typecheck, lint, build выступают проверочными воротами внутри каждой итерации
- Оригинальный подход Huntley -- наблюдение за ошибками цикла как механизм улучшения

**Плюсы:**
- Простота реализации
- Полный контекст задачи доступен при проверке
- Тесты -- объективная мера, не требующая субъективного суждения

**Минусы:**
- Нет "свежего взгляда" -- ревьюер и автор -- один и тот же контекст
- Ограничен механическими проверками (lint, test), не ловит семантические проблемы

**Источники:**
- [The Ralph Playbook | Clayton Farr](https://claytonfarr.github.io/ralph-playbook/)
- [GitHub -- ClaytonFarr/ralph-playbook](https://github.com/ClaytonFarr/ralph-playbook)
- [11 Tips For AI Coding With Ralph Wiggum | AI Hero](https://www.aihero.dev/tips-for-ai-coding-with-ralph-wiggum)

### 3.2. Подход B: Ревью как отдельная итерация (Review as Separate Iteration)

**Описание:** Работа и ревью выполняются в разных итерациях с разными контекстными окнами. Модель A делает работу и пишет в файлы, затем Модель B (или свежий экземпляр модели A) ревьюирует работу в новой сессии.

**Цикл:**
1. Итерация 1 WORK PHASE: Модель A выполняет работу, пишет файлы
2. Итерация 1 REVIEW PHASE: Модель B ревьюирует (свежий контекст!)
3. Вердикт: SHIP (выход) или REVISE (обратная связь записывается в файл)
4. Итерация 2 WORK PHASE: Модель A читает обратную связь (свежий контекст!), исправляет

**Представители:**
- ralphex (umputun) -- 5 параллельных ревью-агентов + опционально GPT-5 cross-validation
- Awesome Claude Ralph Wiggum -- описывает worker/reviewer модель
- bmalph (LarsCowe) -- отдельные сессии для тестов, реализации и ревью

**Плюсы:**
- Свежий контекст для ревьюера -- ловит ошибки, которые "замылились" у работника
- Кросс-модельная валидация (разные модели или разные экземпляры)
- Полностью соответствует философии Ralph ("свежий контекст")

**Минусы:**
- Дополнительные расходы на API-вызовы
- Обратная связь может быть неточной из-за потери контекста задачи
- Более сложная оркестрация

**Источники:**
- [ralphex -- Autonomous Claude Code Loop](https://ralphex.com/)
- [GitHub -- umputun/ralphex](https://github.com/umputun/ralphex)
- [Ralph Wiggum -- AI Loop Technique | Awesome Claude](https://awesomeclaude.ai/ralph-wiggum)
- [Bmalph: BMAD planning + Ralph autonomous loop | DEV Community](https://dev.to/lacow/bmalph-bmad-planning-ralph-autonomous-loop-glued-together-in-one-command-14ka)
- [GitHub -- LarsCowe/bmalph](https://github.com/LarsCowe/bmalph)

### 3.3. Подход C: Параллельные суб-агенты ревью (Parallel Sub-Agent Review)

**Описание:** Вместо одного ревьюера запускается группа специализированных суб-агентов параллельно, каждый из которых фокусируется на отдельном аспекте качества кода.

**Представители:**
- HAMY (Hamilton Greene) -- 9 параллельных суб-агентов (подробнее в разделе 5)
- ralphex -- 5 специализированных ревью-агентов
- Anthropic Code Review Plugin -- множество агентов с confidence scoring

**Плюсы:**
- Многоаспектное покрытие (безопасность, производительность, качество, тесты и т.д.)
- Параллельное выполнение -- не увеличивает время в разы
- Каждый агент получает свежий контекст с узкой задачей

**Минусы:**
- Высокая стоимость API-вызовов (9 параллельных агентов)
- Координация результатов требует основного агента-синтезатора
- Может давать противоречивые рекомендации

**Источники:**
- [9 Parallel AI Agents That Review My Code (Claude Code Setup) | HAMY](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents)
- [How To Run In-Terminal Code Reviews with Claude Code | HAMY](https://hamy.xyz/blog/2025-12_claude-code-review)
- [Claude Code Sub-Agents: Parallel vs Sequential Patterns | claudefa.st](https://claudefa.st/blog/guide/agents/sub-agent-best-practices)
- [GitHub -- VoltAgent/awesome-claude-code-subagents](https://github.com/VoltAgent/awesome-claude-code-subagents)

### 3.4. Подход D: Авто-ревью через хуки (Auto-Review via Hooks)

**Описание:** Код-ревью автоматически запускается через систему хуков Claude Code при каждом завершении работы агента.

**Представители:**
- Nick Tune -- автоматическое ревью через PostToolUse и Stop hooks (подробнее в разделе 6)
- Claude Code Code Review Plugin -- встроенное автоматическое ревью PR

**Плюсы:**
- Ревью происходит автоматически, без ручной оркестрации
- Ловит семантические проблемы, которые не ловят линтеры
- Может блокировать завершение сессии до устранения замечаний

**Минусы:**
- Stop hook не всегда срабатывает надежно (Claude может приостанавливаться для уточняющих вопросов)
- Коммиты могут происходить до завершения ревью
- Ограничен текущим контекстным окном

**Источники:**
- [Auto-Reviewing Claude's Code | Nick Tune (O'Reilly)](https://www.oreilly.com/radar/auto-reviewing-claudes-code/)
- [Auto-Reviewing Claude's Code | Nick Tune (Medium)](https://medium.com/nick-tune-tech-strategy-blog/auto-reviewing-claudes-code-cb3a58d0a3d0)
- [Code Quality Feedback Loops in AI Dev Workflows | Nick Tune](https://nick-tune.me/blog/2026-02-01-code-quality-feedback-loops/)
- [GitHub -- anthropics/claude-code code-review plugin](https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md)

### 3.5. Подход E: Гибридный (Agent Teams + Ralph Loop)

**Описание:** Комбинирование Agent Teams (параллельные AI-"коллеги" с общей доской задач) для креативных решений с Ralph Loop для механической работы.

**Представитель:**
- Meag Tessmann -- гибридная система (~200 строк bash + 270 строк описаний скиллов)

**Принцип:** Ни один паттерн сам по себе не достаточен; вместе они покрывают слабости друг друга:
- Agent Teams хороши для креативной работы (документация, API-дизайн, стратегия тестирования)
- Ralph Loop хорош для механической работы (багфиксы, бойлерплейт, прохождение тестов)
- Ревью запускается между фазами, а не только в конце

**Источники:**
- [When Agent Teams Meet the Ralph Wiggum Loop | Meag Tessmann (Medium)](https://medium.com/@himeag/when-agent-teams-meet-the-ralph-wiggum-loop-4bbcc783db23)
- [Agent Teams Just Shipped in Claude Code | Charles Jones](https://charlesjones.dev/blog/claude-code-agent-teams-vs-subagents-parallel-development)
- [GitHub -- alfredolopez80/multi-agent-ralph-loop](https://github.com/alfredolopez80/multi-agent-ralph-loop)

---

## 4. Нарушает ли "resume" философию Ralph?

### 4.1. Суть вопроса

Claude Code поддерживает флаг `--resume`, который позволяет продолжить предыдущую сессию, восстановив все когнитивное состояние агента. Вопрос: допустимо ли использовать `--resume` для ревью в контексте Ralph Loop?

### 4.2. Аргументы ПРОТИВ resume для ревью

**Нарушение принципа свежего контекста.** Центральная философия Ralph -- каждая итерация получает свежий контекст. `--resume` восстанавливает старую сессию, включая все накопленные ошибки, заблуждения и "шум". Это прямо противоречит принципу "One context window, one activity, one goal".

**Деградация качества.** Как показывает сообщество, после 100K токенов модель входит в "Dumb Zone". Resume продолжает загружать контекст, потенциально привнося деградацию именно в тот момент, когда нужна максимальная точность -- при ревью.

**Позиция Huntley.** Geoffrey Huntley явно разделяет подход Anthropic plugin (persistent context) и свой оригинальный подход (fresh context per iteration). Anthropic plugin, работающий в рамках одной сессии, Huntley не считает "настоящим Ralph Loop".

**Позиция TheTrav.** В статье "The Real Ralph Wiggum Loop: What Everyone Gets Wrong" автор явно указывает: плагин Anthropic использует "single persistent context where tasks accumulate, eventually causing COMPACTION and degradation". Оригинальный подход Geoff'a "spawns fresh context for each iteration".

### 4.3. Аргументы ЗА resume для ревью

**Экономия контекста.** `--resume` избегает "cold start tax" -- затрат времени и токенов на восстановление контекста. Для ревью, где важно понимание задачи, это может быть оправдано.

**Полный контекст решений.** Ревьюер с resume знает ВСЕ решения, принятые при реализации -- почему был выбран определенный подход, какие альтернативы рассматривались.

**Практическая гибкость.** Для небольших задач, где контекст не переполнен, resume может быть более эффективен, чем пересоздание контекста.

### 4.4. Консенсус сообщества

**Общий вердикт: resume для ревью ПРОТИВОРЕЧИТ чистой философии Ralph**, но может быть допустим как прагматическое решение в ограниченных случаях. Сообщество в целом склоняется к тому, что ревью должен выполняться со свежим контекстом -- либо как отдельная итерация, либо через параллельных суб-агентов. Это обеспечивает "свежий взгляд", который является одним из ключевых преимуществ кросс-модельного ревью.

### Источники

- [Claude Code Feature: --resume Is a Game Changer | Adithya Giridharan (Medium)](https://medium.com/@AdithyaGiridharan/the-claude-code-feature-i-slept-on-for-months-and-why-resume-is-a-game-changer-524a21be7061)
- [Ralph Wiggum vs Ralph Loop in Claude Code CLI | Newline](https://www.newline.co/@Dipen/ralph-wiggum-vs-ralph-loop-in-claude-code-cli--ec7625ba)
- [The Real Ralph Wiggum Loop | TheTrav (Substack)](https://thetrav.substack.com/p/the-real-ralph-wiggum-loop-what-everyone)

---

## 5. Подход HAMY: 9 параллельных суб-агентов

### 5.1. Архитектура

Hamilton Greene (HAMY) разработал систему код-ревью, использующую 9 параллельных суб-агентов через Claude Code, каждый из которых фокусируется на отдельном аспекте качества кода.

### 5.2. Девять агентов

| # | Агент | Фокус |
|---|-------|-------|
| 1 | Test Runner | Запуск тестов, отчет о pass/fail с деталями ошибок |
| 2 | Linter & Static Analysis | Линтинг, IDE-диагностика, ошибки типов, неразрешенные ссылки |
| 3 | Code Reviewer | До 5 конкретных улучшений, ранжированных по влиянию и усилию |
| 4 | Security Reviewer | Инъекции, проблемы авторизации, секреты в коде, обработка ошибок |
| 5 | Quality & Style Reviewer | Сложность, мертвый код, дублирование, соответствие конвенциям |
| 6 | Test Quality Reviewer | Покрытие ROI, тестирование поведения, риски flaky-тестов |
| 7 | Performance Reviewer | N+1 запросы, блокирующие операции, утечки памяти |
| 8 | Dependency & Deployment Safety | Новые зависимости, breaking changes |
| 9 | Simplification & Maintainability | Можно ли упростить, проверка атомарности |

### 5.3. Система вердиктов

Основной агент синтезирует результаты всех 9 суб-агентов в приоритизированное резюме с финальным вердиктом:

- **Ready to Merge** -- тесты проходят, нет критических/высоких проблем
- **Needs Attention** -- средние проблемы или важные предложения
- **Needs Work** -- критические/высокие проблемы или падающие тесты

### 5.4. Применение в Ralph Loop

HAMY использует эту систему двояко:
1. Для ревью собственных изменений
2. Как обязательный шаг перед тем, как AI-агент отмечает задачу как "done" -- агент запускает ревью и итерирует по обратной связи

### 5.5. Реакция сообщества

Подход вызвал значительный интерес. Появились аналогичные проекты:
- [VoltAgent/awesome-claude-code-subagents](https://github.com/VoltAgent/awesome-claude-code-subagents) -- коллекция 100+ специализированных суб-агентов
- [lst97/claude-code-sub-agents](https://github.com/lst97/claude-code-sub-agents) -- коллекция для full-stack разработки
- [hamelsmu/claude-review-loop](https://github.com/hamelsmu/claude-review-loop) -- плагин автоматического ревью-цикла с Codex

**Критика:** высокая стоимость (9 параллельных API-вызовов), потенциально избыточно для небольших изменений.

### Источники

- [9 Parallel AI Agents That Review My Code | HAMY](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents)
- [How To Run In-Terminal Code Reviews with Claude Code | HAMY](https://hamy.xyz/blog/2025-12_claude-code-review)
- [Claude Code Sub-Agents Best Practices | PubNub](https://www.pubnub.com/blog/best-practices-for-claude-code-sub-agents/)
- [How to use Claude Code subagents to parallelize development | HN](https://news.ycombinator.com/item?id=45181577)

---

## 6. Подход Nick Tune: авто-ревью через хуки

### 6.1. Механизм

Nick Tune реализовал систему автоматического код-ревью через систему хуков Claude Code:

1. **PostToolUse хук** (matcher: `Write|Edit|MultiEdit`): ведет лог каждого измененного файла
2. **Stop хук**: при завершении работы Claude находит файлы, измененные с момента последнего ревью, и запускает суб-агент для их проверки
3. Суб-агент возвращает ошибку, блокируя основного агента и заставляя его обратить внимание на замечания ревью

### 6.2. Фокус ревью

Tune целенаправленно фокусируется на семантических проблемах, которые не ловят автоматические инструменты:

- **Именование:** замена обобщенных терминов ("helper") на доменно-специфичный язык
- **Утечка логики:** предотвращение выхода бизнес-логики из доменной модели в слои приложения
- **Дефолтные значения:** вызов под сомнение ненужных дефолтов вместо молчаливого "success"

### 6.3. Цикл обратной связи

Tune расширяет подход до полного цикла обратной связи по качеству:

1. **Захват:** документирование всех находок (локальные ревью + GitHub PR)
2. **Анализ:** "5 почему" для каждой находки -- почему проблема не была поймана раньше
3. **Применение:** преобразование инсайтов в конкретные действия:
   - Правила линтинга для раннего обнаружения
   - Dependency cruiser для архитектурных ограничений
   - Документация конвенций для не-автоматизируемых правил
   - Улучшение инструкций агентам и скиллов

### 6.4. Ограничения и предложения

Tune отмечает ненадежность Stop hook -- Claude может приостанавливаться для уточняющих вопросов, а коммиты могут происходить до завершения ревью. Он предлагает Anthropic реализовать хук более высокого уровня -- `CodeReadyForReview` -- привязанный к реальному workflow разработки.

### 6.5. Связь с философией Ralph

Подход Tune работает ВНУТРИ одной сессии (не создает свежий контекст для ревью), что технически отклоняется от чистой философии Ralph. Однако, суб-агент для ревью получает собственный контекст (через Task tool), что частично компенсирует это ограничение.

### Источники

- [Auto-Reviewing Claude's Code | O'Reilly Radar](https://www.oreilly.com/radar/auto-reviewing-claudes-code/)
- [Auto-Reviewing Claude's Code | Nick Tune (Medium)](https://medium.com/nick-tune-tech-strategy-blog/auto-reviewing-claudes-code-cb3a58d0a3d0)
- [Code Quality Feedback Loops in AI Dev Workflows | Nick Tune](https://nick-tune.me/blog/2026-02-01-code-quality-feedback-loops/)
- [Minimalist Claude Code Task Management Workflow | Nick Tune](https://medium.com/nick-tune-tech-strategy-blog/minimalist-claude-code-task-management-workflow-7b7bdcbc4cc1)
- [Nick Tune LinkedIn post](https://www.linkedin.com/posts/nick-tune_ive-been-playing-around-with-the-new-claude-activity-7347279569286012928-AV97)

---

## 7. Реализации и форки: как они обрабатывают ревью

### 7.1. ralphex (umputun)

**Подход:** Полноценная мульти-агентная система ревью. После выполнения каждой задачи запускаются 5 специализированных агентов параллельно:

| Агент | Фокус |
|-------|-------|
| Quality | Баги, безопасность, race conditions, обработка ошибок |
| Implementation | Верификация достижения заявленных целей |
| Testing | Покрытие тестами, edge cases, качество тестов |
| Simplification | Over-engineering, ненужная сложность |
| Documentation | README, комментарии, документация |

Опциональное GPT-5 ревью для независимого анализа. Каждая задача выполняется в новой сессии Claude -- нет деградации контекста.

**Источники:**
- [ralphex.com](https://ralphex.com/)
- [GitHub -- umputun/ralphex](https://github.com/umputun/ralphex)
- [RALPHEX Documentation](https://ralphex.com/docs/)

### 7.2. snarktank/ralph

**Подход:** Ревью реализуется через автоматическую верификацию. После каждой итерации Ralph обновляет файлы AGENTS.md с "уроками", которые AI автоматически читает в будущих итерациях. Память сохраняется через git-историю, progress.txt и prd.json.

**Источники:**
- [GitHub -- snarktank/ralph](https://github.com/snarktank/ralph)

### 7.3. frankbria/ralph-claude-code

**Подход:** Акцент на безопасности: dual-condition exit gate (требуется и индикатор завершения, И явный EXIT_SIGNAL), rate limiting (100 вызовов/час), circuit breaker. Ревью -- через автоматическую валидацию внутри итерации.

**Источники:**
- [GitHub -- frankbria/ralph-claude-code](https://github.com/frankbria/ralph-claude-code)

### 7.4. bmalph (LarsCowe)

**Подход:** Разделение планирования (BMAD) и реализации (Ralph). Ralph запускает отдельные сессии для разных слоев: тесты, реализация, ревью. Ключевое преимущество -- планировочные документы как постоянный контекст: "когда реализуется story 8, агент все еще знает, что было решено в story 1, потому что документы планирования доступны."

**Источники:**
- [Bmalph | DEV Community](https://dev.to/lacow/bmalph-bmad-planning-ralph-autonomous-loop-glued-together-in-one-command-14ka)
- [GitHub -- LarsCowe/bmalph](https://github.com/LarsCowe/bmalph)

### 7.5. open-ralph-wiggum (Th0rgal)

**Подход:** Универсальный инструмент для запуска Ralph Loop с разными AI-агентами (Open Code, Claude Code, Codex, Copilot). Минималистичный подход -- фокус на цикле, ревью остается на усмотрение пользователя.

**Источники:**
- [GitHub -- Th0rgal/open-ralph-wiggum](https://github.com/Th0rgal/open-ralph-wiggum)

### 7.6. Официальный Ralph Wiggum Plugin (Anthropic)

**Подход:** Stop hook перехватывает выход сессии и переподает промпт. Работает в рамках ОДНОЙ сессии (persistent context). Geoffrey Huntley и многие в сообществе НЕ СЧИТАЮТ это настоящим Ralph Loop из-за отсутствия свежего контекста.

**Источники:**
- [Claude Code Ralph Wiggum | claudefa.st](https://claudefa.st/blog/guide/mechanics/ralph-wiggum-technique)
- [Ralph Loop plugin issue | GitHub](https://github.com/anthropics/claude-code/issues/16560)

---

## 8. Дискуссии на Hacker News и Reddit

### 8.1. "What Ralph Wiggum Loops Are Missing" (HN)

Ключевые критические замечания из обсуждения:

1. **Отсутствие комплексного планирования.** Ralph Loop пропускает критические соображения -- безопасность, моделирование данных, производительность -- без осознания проблем до их проявления.
2. **Проблемы координации задач.** Без правильной координации агенты могут мешать работе друг друга.
3. **Обработка сложных целей.** Агенты хорошо справляются с изолированными задачами, но сталкиваются с трудностями при взаимосвязанных подцелях.
4. **Коммодитизация.** Участники отметили, что Ralph Loop представляет community-discovered patterns, которые быстро впитываются коммерческими AI-платформами.

**Предложенное решение:** Мульти-агентная верификация -- "мой PRD проверяется другим агентом прежде чем я начну разбивать его на задачи."

### 8.2. Дискуссия о том, "что все делают неправильно"

Codacy, Sparkry AI, Medium/Vibe Coding и другие публикации сходятся в критике:

- **Anthropic plugin -- не настоящий Ralph.** Плагин использует persistent context, что приводит к compaction и деградации.
- **Runtime-фильтрация задач ненадежна (70--80%).** Правильный подход -- scope plans upfront.
- **Huntley отверг популярные реализации.** Когда Ryan Carson опубликовал руководство, Huntley ответил тремя словами: "this isn't it."

### 8.3. Позиция Huntley по код-ревью

Huntley высказал провокационный тезис: **разработчики должны тратить больше времени на создание циклов, которые улучшают вывод AI, а не упорствовать в код-ревью.** Это означает смену парадигмы: от превентивного ревью к системному улучшению процесса генерации.

### Источники

- [What Ralph Wiggum Loops Are Missing | HN](https://news.ycombinator.com/item?id=46750937)
- [What Everyone Gets Wrong About The Ralph Loop | Codacy](https://blog.codacy.com/what-everyone-gets-wrong-about-the-ralph-loop)
- [Everyone's Using Ralph Loops Wrong | Sparkry AI](https://sparkryai.substack.com/p/everyones-using-ralph-loops-wrong)
- [Self-Improving Coding Agents | Addy Osmani](https://addyosmani.com/blog/self-improving-agents/)

---

## 9. Связь между принципом свежего контекста и код-ревью

### 9.1. Ключевое разделение

Документация DeepWiki (claude-code-ultimate-guide) явно разделяет два паттерна:

| Характеристика | Ralph Loop | Iterative Refinement |
|---------------|-----------|---------------------|
| Контекст | Свежий каждую итерацию | Непрерывный |
| Состояние | Файловая система | Контекст LLM |
| Оркестрация | Bash | /pr commands |
| Назначение | Реализация | Код-ревью и quality gates |
| Ревью | Автоматическое (тесты) | Мульти-агентное |

### 9.2. Рекомендация сообщества

Преобладающий консенсус: **используйте Ralph Loop для реализации, используйте Iterative Refinement для код-ревью и quality gates.** Ревью работает лучше с непрерывным контекстом (или специализированными суб-агентами), а не с тем же паттерном свежего контекста, который оптимален для реализации.

### 9.3. Исключение: суб-агенты

Суб-агенты (как у HAMY и ralphex) получают собственный свежий контекст для каждого ревью, но этот контекст целенаправленно минимален -- только diff изменений и конкретное задание. Это создает "золотую середину": свежий контекст для ревьюера, но достаточно информации для качественного анализа.

---

## 10. Сравнительная таблица подходов

| Подход | Свежий контекст ревью | Соответствие Ralph | Глубина ревью | Стоимость | Автоматизация |
|--------|----------------------|-------------------|---------------|-----------|---------------|
| A: Внутри итерации | Нет | Частично | Механическое | Низкая | Высокая |
| B: Отдельная итерация | Да | Полное | Семантическое | Средняя | Высокая |
| C: Параллельные суб-агенты | Да (каждый агент) | Полное | Глубокое, многоаспектное | Высокая | Высокая |
| D: Авто-ревью через хуки | Частично (суб-агент) | Частично | Семантическое | Средняя | Очень высокая |
| E: Гибрид (Teams + Ralph) | Да | Полное | Креативное + механическое | Высокая | Средняя |
| Resume для ревью | Нет | Нарушает | Полное (все решения) | Низкая | Средняя |

---

## 11. Выводы и рекомендации

### 11.1. Консенсус сообщества (февраль 2026)

1. **Ревью НЕОБХОДИМ в Ralph Loop.** Автономная генерация кода без проверки -- анти-паттерн. Как пишет Huntley: "If a loop can touch your authentication layer, rewrite your database access patterns, or refactor your API without a human reviewing every line, you need something watching that code before it ships."

2. **Свежий контекст для ревью предпочтителен.** Ревью со свежим контекстом (подходы B, C, E) соответствует философии Ralph и обеспечивает "свежий взгляд". Resume для ревью нарушает эту философию.

3. **Параллельные суб-агенты -- наиболее продвинутый подход.** HAMY (9 агентов) и ralphex (5 агентов) демонстрируют тренд к мульти-аспектному ревью с параллельным выполнением.

4. **Авто-ревью через хуки -- перспективное направление.** Nick Tune показал, что семантическое ревью может быть автоматизировано через систему хуков, но технология еще не созрела (ненадежность Stop hook).

5. **Гибридные подходы побеждают.** Meag Tessmann и другие показали, что комбинирование Agent Teams (для креативных решений) с Ralph Loop (для механической работы) дает лучшие результаты.

6. **Git + PR как финальный gate.** Даже с автоматическим ревью внутри цикла, многие практики рекомендуют открывать PR в конце (а не автоматически мерджить), чтобы человек мог провести финальную проверку.

### 11.2. Открытые вопросы

- Нет единого стандарта на формат обратной связи между ревью-итерациями
- Оптимальное количество ревью-агентов не определено (варьируется от 1 до 9)
- Баланс между стоимостью API и глубиной ревью остается индивидуальным решением
- Anthropic может реализовать более надежные хуки (CodeReadyForReview), что изменит ландшафт

### 11.3. Рекомендация для bmad-ralph

На основании анализа сообщества, рекомендуемый подход для bmad-ralph:

1. **Ревью как отдельная итерация со свежим контекстом** (подход B) как основной паттерн
2. **Мульти-агентное параллельное ревью** (подход C) для критических задач -- от 3 до 5 специализированных агентов
3. **Автоматическая валидация внутри итерации** (подход A) как первый фильтр (тесты, lint, typecheck)
4. **Human gate через PR** как финальный контроль
5. **НЕ использовать resume для ревью** -- это противоречит философии Ralph и снижает качество за счет деградации контекста

---

## Полный список источников

### Оригинальные работы Geoffrey Huntley
- [Everything is a Ralph Loop](https://ghuntley.com/loop/)
- [Ralph Wiggum as a Software Engineer](https://ghuntley.com/ralph/)
- [GitHub -- ghuntley/how-to-ralph-wiggum](https://github.com/ghuntley/how-to-ralph-wiggum)

### Ключевые публикации и анализ
- [Inventing the Ralph Wiggum Loop | LinearB Podcast](https://linearb.io/dev-interrupted/podcast/inventing-the-ralph-wiggum-loop)
- [Mastering Ralph Loops | LinearB Blog](https://linearb.io/blog/ralph-loop-agentic-engineering-geoffrey-huntley)
- [What Everyone Gets Wrong About The Ralph Loop | Codacy](https://blog.codacy.com/what-everyone-gets-wrong-about-the-ralph-loop)
- [The Real Ralph Wiggum Loop | TheTrav](https://thetrav.substack.com/p/the-real-ralph-wiggum-loop-what-everyone)
- [Everyone's Using Ralph Loops Wrong | Sparkry AI](https://sparkryai.substack.com/p/everyones-using-ralph-loops-wrong)
- [Ralph Loops: What Most Developers Get Wrong | Vibe Coding](https://medium.com/vibe-coding/everyones-using-ralph-loops-wrong-here-s-what-actually-works-e5e4208873c1)
- [Ralph Wiggum, Explained | Dev Genius](https://blog.devgenius.io/ralph-wiggum-explained-the-claude-code-loop-that-keeps-going-3250dcc30809)

### Код-ревью подходы
- [9 Parallel AI Agents That Review My Code | HAMY](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents)
- [How To Run In-Terminal Code Reviews | HAMY](https://hamy.xyz/blog/2025-12_claude-code-review)
- [Auto-Reviewing Claude's Code | Nick Tune (O'Reilly)](https://www.oreilly.com/radar/auto-reviewing-claudes-code/)
- [Auto-Reviewing Claude's Code | Nick Tune (Medium)](https://medium.com/nick-tune-tech-strategy-blog/auto-reviewing-claudes-code-cb3a58d0a3d0)
- [Code Quality Feedback Loops | Nick Tune](https://nick-tune.me/blog/2026-02-01-code-quality-feedback-loops/)
- [When Agent Teams Meet the Ralph Loop | Meag Tessmann](https://medium.com/@himeag/when-agent-teams-meet-the-ralph-wiggum-loop-4bbcc783db23)

### Реализации и форки
- [ralphex](https://ralphex.com/) | [GitHub](https://github.com/umputun/ralphex)
- [snarktank/ralph | GitHub](https://github.com/snarktank/ralph)
- [frankbria/ralph-claude-code | GitHub](https://github.com/frankbria/ralph-claude-code)
- [bmalph | DEV Community](https://dev.to/lacow/bmalph-bmad-planning-ralph-autonomous-loop-glued-together-in-one-command-14ka) | [GitHub](https://github.com/LarsCowe/bmalph)
- [Th0rgal/open-ralph-wiggum | GitHub](https://github.com/Th0rgal/open-ralph-wiggum)
- [alfredolopez80/multi-agent-ralph-loop | GitHub](https://github.com/alfredolopez80/multi-agent-ralph-loop)
- [The Ralph Playbook | Clayton Farr](https://claytonfarr.github.io/ralph-playbook/) | [GitHub](https://github.com/ClaytonFarr/ralph-playbook)
- [awesome-ralph | GitHub](https://github.com/snwfdhmp/awesome-ralph)

### Технические руководства и документация
- [Fresh Context Pattern (Ralph Loop) | DeepWiki](https://deepwiki.com/FlorianBruniaux/claude-code-ultimate-guide/7.3-fresh-context-pattern-(ralph-loop))
- [Ralph Loop | Goose](https://block.github.io/goose/docs/tutorials/ralph-loop/)
- [Create Custom Subagents | Claude Code Docs](https://code.claude.com/docs/en/sub-agents)
- [Automate Workflows with Hooks | Claude Code Docs](https://code.claude.com/docs/en/hooks-guide)
- [Claude Code Code Review Plugin | GitHub](https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md)
- [awesome-claude-code-subagents | GitHub](https://github.com/VoltAgent/awesome-claude-code-subagents)

### Обзорные и аналитические статьи
- [2026: The Year of the Ralph Loop Agent | DEV Community](https://dev.to/alexandergekov/2026-the-year-of-the-ralph-loop-agent-1gkj)
- [A Brief History of Ralph | HumanLayer](https://www.humanlayer.dev/blog/brief-history-of-ralph)
- [VentureBeat -- Ralph Wiggum from Simpsons to AI](https://venturebeat.com/technology/how-ralph-wiggum-went-from-the-simpsons-to-the-biggest-name-in-ai-right-now)
- [The Register -- Ralph Wiggum Loop](https://www.theregister.com/2026/01/27/ralph_wiggum_claude_loops/)
- [From ReAct to Ralph Loop | Alibaba Cloud](https://www.alibabacloud.com/blog/from-react-to-ralph-loop-a-continuous-iteration-paradigm-for-ai-agents_602799)
- [Self-Improving Coding Agents | Addy Osmani](https://addyosmani.com/blog/self-improving-agents/)
- [Ralph Loop with Google ADK | Google Cloud Community](https://medium.com/google-cloud/ralph-loop-with-google-adk-ai-agents-that-verify-not-guess-b41f71c0f30f)

### Обсуждения
- [What Ralph Wiggum Loops Are Missing | Hacker News](https://news.ycombinator.com/item?id=46750937)
- [How to use Claude Code subagents to parallelize development | Hacker News](https://news.ycombinator.com/item?id=45181577)
