# Non-Functional Requirements

### Performance

- **NFR1:** Общая утилизация context window за execute-сессию не должна превышать 40-50%. После 50% модель теряет начальные инструкции — промпт, guardrails, 999-правила (Liu et al. 2024 "Lost in the Middle", NoLiMa/Adobe 2025, ClaudeLog 2025). Обеспечивается через: (a) fresh context на каждую задачу, (b) `--max-turns` как hard limit, (c) Serena для token-efficient чтения кода, (d) компактные контекстные файлы. При превышении суммарного размера контекстных файлов порога — ralph выводит warning
- **NFR2:** Overhead самого ralph (loop, парсинг sprint-tasks.md, запуск Claude) — не более 5 секунд между итерациями. Узкое место — Claude API, не ralph
- **NFR3:** При batch review ralph проверяет размер cumulative diff. Если превышает порог (~120K символов ≈ ~30K токенов), выводит warning с рекомендацией уменьшить review-every. Автоматическое разбиение diff — Growth feature

### Security

- **NFR4:** Ralph никогда не удаляет файлы пользователя и не выполняет `git reset --hard` или `git push --force` без явного human gate. Деструктивные git-операции запрещены в execute-промпте через 999-правила
- **NFR5:** Config-файл и промпты не содержат и не передают API-ключи. Claude CLI использует свой собственный auth
- **NFR6:** Ralph вызывает Claude Code через CLI с флагом `--dangerously-skip-permissions` для автономного выполнения. Безопасность обеспечивается не permission-системой Claude, а guardrails в промпте (999-правила), ATDD (тесты как backpressure), review-агентами и human gates

### Integration

- **NFR7:** Ralph использует документированные CLI-параметры Claude Code: `-p` (prompt), `--max-turns`, `--allowedTools`, `--dangerously-skip-permissions`. Никаких внутренних API или недокументированных флагов
- **NFR8:** Serena интеграция — best effort. Любой сбой Serena (таймаут, индексация не завершена, MCP недоступен) приводит к graceful fallback на стандартное чтение файлов, а не к ошибке
- **NFR9:** Ralph работает с любым git-репозиторием. Никаких предположений о структуре проекта, языке программирования или фреймворке

### Reliability

- **NFR10:** При аварийном завершении (kill, crash, потеря питания) ralph корректно возобновляет работу через `ralph run` — продолжая с первой незавершённой задачи. sprint-tasks.md = single source of state
- **NFR11:** Успешная итерация атомарна: commit происходит при green tests. WIP-коммиты допускаются только через resume-extraction при незавершённом execute (для сохранения прогресса). Промежуточное состояние (dirty working tree без commit) допустимо только внутри active execute-сессии
- **NFR12:** При сбое Claude CLI (API таймаут, rate limit, exit code != 0) ralph делает retry с exponential backoff. После N неудач (configurable, default 3) — останавливается с информативным сообщением и exit code
- **NFR13:** Graceful shutdown: при Ctrl+C ralph дожидается завершения текущей Claude-сессии (или убивает её по второму Ctrl+C) и выходит чисто. sprint-tasks.md не требует обновления — незавершённая задача остаётся `[ ]` (review ещё не подтвердил качество), при resume ralph подхватит её заново
- **NFR14:** Лог-файл `.ralph/logs/run-YYYY-MM-DD-HHMMSS.log` — append-only запись всех событий: старт/стоп задач, результаты тестов, review findings, human gate решения, ошибки. Для post-mortem анализа после длительных запусков
- **NFR15:** Knowledge files: LEARNINGS.md — append-only с hard limit (при превышении бюджета — distillation-сессия сжимает: оставляет ценное, убирает дублирование и устаревшее). CLAUDE.md секция ralph — обновляется review (при findings) и resume-extraction (при неуспехе): add/rewrite/deduplicate. review-findings.md — транзиентный файл для текущей задачи: перезаписывается review при findings, очищается при clean review

### Portability

- **NFR16:** Ralph распространяется как single Go binary. Поддержка: Linux, macOS (нативно), Windows (через WSL). Кросс-компиляция через `GOOS/GOARCH`
- **NFR17:** Единственные hard dependencies для пользователя: `git`, `claude` CLI. Ralph — single Go binary, zero runtime dependencies. Установка: `go install` или скачать binary из GitHub Releases

### Maintainability

- **NFR18:** Все промпты для Claude (execute, review agents, distillation) хранятся как отдельные текстовые файлы. Defaults встроены в binary через `go:embed`, кастомные файлы в `.ralph/agents/` имеют приоритет (fallback chain: project → global → embedded). Изменение промпта не требует пересборки
- **NFR19:** Добавление нового review-агента = добавление `.md` файла в директорию агентов. Не требует изменения кода ralph
- **NFR20:** Конкретная файловая структура компонентов определяется на этапе архитектуры. NFR-критерий: каждый компонент (bridge, runner, session, gates, config) — изолированная единица с минимальными зависимостями между ними
