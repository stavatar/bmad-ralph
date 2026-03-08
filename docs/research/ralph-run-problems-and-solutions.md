# Проблемы Ralph Run и принятые решения

> Документ по результатам анализа Ralph Run на проекте mentorlearnplatform (2026-03-06).
> Основан на: метриках 3 ранов, логах сессий, исследованиях архитектора и аналитика.

## Контекст

Ralph Run был запущен на проекте mentorlearnplatform для выполнения 15 задач из sprint-tasks.md.
За 3 рана были выявлены 4 системные проблемы, для каждой из которых согласованы решения.

### Метрики тестовых ранов

| Ран | Задач | Стоимость | Время | Сессий | Проблемы |
|-----|-------|-----------|-------|--------|----------|
| 1 (738ec707) | 0 выполнено | ~$1.70 | ~5 мин | 2 | Задача уже выполнена, stuck |
| 2 (9271dc4d) | 1 выполнено | ~$3.50 | ~15 мин | 6 | Scope creep, задачи уже выполнены |
| 3 (422cb60f) | 5 выполнено | $38.44 | 132 мин | 22 | Гидра на BF-3.3 ($20.29), distillation path bug |

---

## Проблема 1: Повторное выполнение уже завершённых задач

### Суть

Ralph берёт задачу `[ ]` из sprint-tasks.md, запускает execute, Claude тратит $1-2 и обнаруживает, что задача уже закоммичена. Причина — sprint-tasks.md был перегенерирован без пометок `[x]`, а Ralph не проверяет git history перед началом.

### Обсуждение

- Ситуация перегенерации sprint-tasks.md редкая, но возможная (bridge re-run, smart merge)
- Простая проверка по git log ненадёжна: коммит != выполненная задача (может быть закоммичено, но не пройти review)
- Commit message формируется Claude свободно — нет маркеров для матчинга
- Нужна связка: маркер в коммите + проверка отсутствия findings

### Принятое решение

**Два компонента:**

**A. Smart merge сохраняет `[x]` при перегенерации sprint-tasks.md**
- При bridge re-run пометки `[x]` из предыдущей версии переносятся в новую
- Профилактика: не допускаем рассинхрон

**B. Pre-flight проверка перед execute**
- Ralph добавляет в execute.md требование включать маркер `[task:<хэш>]` в commit message
- Хэш вычисляется из текста задачи (первые 6 символов SHA-256)
- Формат: `[task:a1b2c3]` в конце commit message
- Pre-flight алгоритм:
  1. Берём задачу `[ ]`
  2. Вычисляем хэш текста задачи
  3. Ищем `[task:<хэш>]` в `git log --oneline -20`
  4. Если найден — проверяем отсутствие `review-findings.md` с незакрытыми замечаниями
  5. Коммит есть + findings нет → пропускаем, помечаем `[x]`
  6. Коммит есть + findings есть → стандартный цикл execute (нужно исправить замечания)
  7. Коммит не найден → стандартный цикл execute

### Почему хэш, а не другие варианты

| Вариант | Плюсы | Минусы | Решение |
|---------|-------|--------|---------|
| Хэш текста задачи `[task:a1b2c3]` | Компактно, точный матчинг, не зависит от порядка строк | Человеку непонятно какая задача | **Выбран** |
| Порядковый номер `[task:3]` | Просто | Номер сдвигается при перегенерации | Отклонён |
| Текст задачи в commit message | Человекочитаемо | Длинно, может обрезаться | Отклонён |

---

## Проблема 2: WSL/Windows path баг в дистилляции

### Суть

`AutoDistill` вызывает `os.ReadFile` с путём `/mnt/e/Projects/...`, но Windows Go (`go.exe`) конвертирует его в `E:\Projects\...`. Ошибка: `open E:\Projects\mentorlearnplatform\LEARNINGS.md: The system cannot find the path specified.`

### Обсуждение

- Ralph должен работать кросс-платформенно: Windows, WSL, Linux, (возможно Mac)
- Два подхода: A) явный конвертер путей `/mnt/c/` <-> `C:\`, B) использовать `filepath.Abs`/`filepath.Join` везде
- Подход A хрупкий — зависит от нестандартных монтирований
- Подход B универсальный — Go stdlib уже знает про текущую OS

### Принятое решение

**Подход B — работать с путями as-is через `filepath.Abs` и `filepath.Join`**

1. Заменить все строковые конкатенации путей на `filepath.Join`
2. Использовать `filepath.Abs` для нормализации путей из разных источников
3. Graceful degradation: при ошибке файловых операций (дистилляция, learnings) — skip с логированием, не abort
4. Дистилляция — не критическая операция, её отказ не должен блокировать run

### Где менять

- `runner/knowledge_distill.go` — guard на `os.IsNotExist` в `AutoDistill`
- Все места конструирования путей — заменить на `filepath.Join`
- `runner/runner.go` — `detectProjectRoot` использует `filepath.Abs`

---

## Проблема 3: Гидра-паттерн (ревью порождает новые замечания)

### Суть

На задаче BF-3.3 было 5 циклов review-fix (findings: 3→2→4→1→0), стоимость $20.29. Review каждый раз находит "новые" замечания — LLM каждый раз смотрит через другую "линзу".

### Исследование

Аналитик провёл deep research с анализом 10+ источников:

**Корневые причины:**
1. Недетерминированность LLM review — один и тот же код при двух запусках даёт разные замечания
2. Scope drift — review проверяет весь diff задачи, включая исправления, расширяя поверхность для замечаний
3. Severity filtering не реализован — `ReviewMinSeverity` есть в конфиге, но не используется в runner
4. Нет контекста предыдущих замечаний — review не знает историю
5. Эскалация слишком слабая — при гидре только смена модели

**Ключевые находки из индустрии:**
- Spotify Honk: разделяет детерминированные проверки (тесты, линт) и субъективные (LLM). Объективные сходятся по определению
- Atomic Robot: итеративное уточнение деградирует — "momentum" фрейминга, убывающая отдача
- IEEE-ISTAS 2025: 37.6% рост критических уязвимостей после 5 итераций LLM refinement
- MIT TACL: intrinsic self-correction не улучшает или ухудшает производительность без внешнего feedback

### Принятое решение

**Комбинация 3 стратегий: прогрессивный порог (S2) + scope lock (S4) + бюджет замечаний (S5)**

#### Прогрессивная схема на 6 циклов

| Цикл | Severity порог | Scope | Макс. замечаний | Модель execute |
|------|---------------|-------|-----------------|----------------|
| 1 | LOW+ (всё) | Полный diff | 5 | Стандартная |
| 2 | LOW+ (всё) | Полный diff | 5 | Стандартная |
| 3 | MEDIUM+ | Инкрементальный diff + контекст | 3 | Макс. модель + EFFORT_LEVEL=high |
| 4 | HIGH+ | Инкрементальный diff + контекст | 1 | Макс. модель + EFFORT_LEVEL=high |
| 5 | CRITICAL | Инкрементальный diff + контекст | 1 | Макс. модель + EFFORT_LEVEL=high |
| 6 | CRITICAL | Инкрементальный diff + контекст | 1 | Макс. модель + EFFORT_LEVEL=high |

- Дефолт `max_review_iterations: 6`
- Extended thinking активируется через `CLAUDE_CODE_EFFORT_LEVEL=high` (переменная окружения, не промпт)

#### Инкрементальный diff (scope lock)

- Циклы 1-2: полный diff задачи (`git diff <before_task>..HEAD`)
- Циклы 3+: инкрементальный diff (`git diff HEAD~1..HEAD`) + контекст:
  - Полное описание задачи + story (для понимания "что надо было сделать")
  - Предыдущие findings (что было найдено в прошлом цикле)
  - Инструкция: "проверь что исправления корректны и не внесли новых проблем уровня <порог>+"

#### Статистика по review агентам

- Добавить 5-е поле `Агент` в формат findings: `- **Агент**: implementation`
- Ralph при парсинге findings извлекает severity + agent
- Агрегация в JSON-лог рана (`.ralph/logs/ralph-run-<id>.json`):

```json
"agent_stats": {
  "quality": {"critical": 0, "high": 2, "medium": 5, "low": 3},
  "implementation": {"critical": 0, "high": 1, "medium": 3, "low": 0},
  "simplification": {"critical": 0, "high": 0, "medium": 2, "low": 4},
  "design-principles": {"critical": 0, "high": 0, "medium": 1, "low": 2},
  "test-coverage": {"critical": 0, "high": 3, "medium": 2, "low": 1}
}
```

### Отклонённые стратегии

| Стратегия | Причина отклонения |
|-----------|-------------------|
| S1: Severity Filtering (FR16a) — сменить дефолт на MEDIUM | Пользователь хочет исправлять все замечания в первых циклах, прогрессивный порог решает проблему лучше |
| S6: Task Budget Cap | Не выбран как отдельная стратегия, но hard stop существует через max_review_iterations=6 |
| S3: Previous Findings в промпте | Частично реализуется через scope lock — на циклах 3+ review получает предыдущие findings как контекст |

---

## Проблема 5: Scope creep (Claude реализует соседнюю задачу)

### Суть

При execute задачи BF-2.2 (переименование кнопок) Claude реализовал BF-2.3 (подсветка строк) и закоммитил. Он видит весь sprint-tasks.md и "перескакивает" на другую задачу.

### Обсуждение

- Скрыть соседние задачи от Claude — потеря контекста проекта
- Claude должен видеть sprint-tasks.md для пометки `[x]` и понимания общей картины
- Лучше усилить инструкции + добавить проверку на review

### Принятое решение

**Двойная защита: жёсткий промпт + review проверка**

#### 1. Усиление execute.md

Добавить блок:

```
## SCOPE BOUNDARY (MANDATORY)

Реализуй ТОЛЬКО текущую задачу: __TASK__
НЕ реализуй другие задачи из sprint-tasks.md, даже если они кажутся связанными.
Если текущая задача зависит от другой — остановись и сообщи, не делай обе.

Перед коммитом проверь: каждый изменённый файл и каждое изменение
напрямую связаны с текущей задачей. Если обнаружишь изменения для другой
задачи — откати их через git checkout.
```

#### 2. Проверка scope в implementation review agent

Добавить в `runner/prompts/agents/implementation.md` пункт:

- Проверить что ВСЕ изменения в diff относятся к AC текущей задачи
- Если обнаружены изменения, реализующие другую задачу из sprint-tasks.md — это HIGH finding
- Формулировка: "Scope creep: изменения в <файл> реализуют задачу '<другая задача>', а не текущую"

---

## Дополнительные решения (выявлены в процессе)

### Подключение FR16a (Severity Filtering)

`ReviewMinSeverity` существует в `config.Config`, но не используется в runner. Хотя как отдельная стратегия он отклонён (прогрессивный порог лучше), сам механизм фильтрации по severity необходим для реализации прогрессивной схемы. Нужно:
- Реализовать `filterBySeverity(findings, minSeverity)` в runner
- Использовать в `DetermineReviewOutcome` с динамическим порогом от номера цикла

### Дефолт max_review_iterations

Изменить с 3 на 6 в `config/defaults.yaml`.

---

## Источники

### Исследование аналитика (гидра-паттерн)

- [IEEE-ISTAS 2025: Security Degradation in Iterative AI Code Generation](https://arxiv.org/html/2506.11022)
- [MIT TACL: When Can LLMs Actually Correct Their Own Mistakes?](https://direct.mit.edu/tacl/article/doi/10.1162/tacl_a_00713/125177)
- [Spotify Engineering: Background Coding Agents — Feedback Loops (Honk, Part 3)](https://engineering.atspotify.com/2025/12/feedback-loops-background-coding-agents-part-3)
- [Atomic Robot: A Two-Phase AI Code Review Workflow](https://atomicrobot.com/blog/ai-refine-then-fresh-perspective/)
- [CodeScene: Agentic AI Coding Best Practice Patterns](https://codescene.com/blog/agentic-ai-coding-best-practice-patterns-for-speed-with-quality)
- [Cognition: Devin Autofixes Review Comments](https://cognition.ai/blog/closing-the-agent-loop-devin-autofixes-review-comments)

### Исследование архитектора

- Анализ `runner/runner.go`, `config/config.go`, `runner/prompts/execute.md`
- Предложения по pre-flight, path normalization, execute.md hardening
