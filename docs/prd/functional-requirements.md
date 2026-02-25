# Функциональные требования

### Планирование задач (Bridge)

- **FR1:** Разработчик может конвертировать BMad story-файлы в структурированный sprint-tasks.md
- **FR2:** Система выводит тест-кейсы из объективных acceptance criteria в stories. Субъективные AC помечаются для ручной или LLM-as-Judge верификации (post-MVP)
- **FR3:** Система определяет и размечает точки human gate тегом `[GATE]` в строке задачи sprint-tasks.md (первая задача epic'а, user-visible milestones). Ralph сканирует `[GATE]` для определения остановочных точек
- **FR4:** Разработчик может повторно запустить bridge с Smart Merge для обновления существующего sprint-tasks.md
- **FR5:** Система генерирует служебные задачи на основе контекста stories. Типовые категории: (a) **project setup** — когда story подразумевает новые зависимости, фреймворки или инфраструктуру (install deps, test config, scaffold); (b) **integration verification** — когда несколько задач работают над разными частями одной фичи (backend + frontend); (c) **e2e checkpoint** (Growth, вместе с batch review) — перед batch review (review-every N) для раннего обнаружения регрессий. Bridge определяет необходимость по сигналам из stories
- **FR5a:** Каждая задача в sprint-tasks.md содержит поле `source:` со ссылкой на оригинальную story и AC (например `source: stories/auth.md#AC-3`). Обеспечивает трассировку задачи → story для correct flow, review и аудита

### Автономное выполнение (Run)

- **FR6:** Система последовательно выполняет задачи из sprint-tasks.md в цикле. При старте `ralph run` — проверка git health (clean state, не detached HEAD, не в merge/rebase)
- **FR7:** Каждое выполнение задачи происходит в свежей сессии Claude Code
- **FR8:** Execute-сессия читает задачу, реализует код, запускает unit-тесты, коммитит при green. e2e запускается только для UI-задач и на checkpoint-ах `review-every N`
- **FR9:** Система повторяет неудачные задачи до настраиваемого максимума итераций. Ralph определяет успешность execute по наличию нового git коммита: есть коммит → переход к review, нет коммита → resume-extraction → retry (execute_attempts++). Resume-extraction возобновляет execute-сессию (`claude --resume`), коммитит WIP, пишет прогресс в sprint-tasks.md и знания в LEARNINGS.md. Два независимых счётчика: `execute_attempts` (max_iterations, default 3) и `review_cycles` (max_review_iterations, default 3)
- **FR10:** Разработчик может ограничить количество ходов Claude Code за одну execute-сессию
- **FR11:** Execute-сессия Claude сама читает sprint-tasks.md и берёт первую невыполненную задачу (`- [ ]`) сверху вниз (модель Playbook — Claude self-directing). Review-сессия отмечает задачу выполненной (`[x]`) после подтверждения качества (clean review без critical findings). Execute-сессии НЕ изменяют статус задач. Ralph сканирует файл (grep `- [ ]`) только для контроля loop — есть ли ещё задачи. Ralph не извлекает описание задач и не передаёт их в промпт
- **FR12:** При повторном запуске `ralph run` система продолжает с первой незавершённой задачи в sprint-tasks.md. При обнаружении dirty working tree (прерванная сессия) — `git checkout -- .` для восстановления чистого состояния перед retry. Мягкая валидация: если sprint-tasks.md не содержит ни `- [ ]`, ни `- [x]` — warning с рекомендацией проверить файл

### Ревью кода

- **FR13:** Система запускает фазу ревью после каждой выполненной задачи
- **FR14:** Ревью выполняется в свежей сессии Claude Code, отдельной от execute
- **FR15:** Review-сессия запускает 4 параллельных sub-агента через Task tool (quality, implementation, simplification, test-coverage)
- **FR16:** Review-сессия верифицирует каждую находку sub-агентов и классифицирует как CONFIRMED или FALSE POSITIVE. Каждый finding получает severity (CRITICAL/HIGH/MEDIUM/LOW)
- **FR16a (Growth):** Findings с severity ниже настроенного порога (`review_min_severity`, default HIGH) записываются в лог, но не блокируют pipeline и не попадают в review-findings.md
- **FR17:** Review-сессия ТОЛЬКО анализирует — при clean review ставит `[x]` и очищает review-findings.md, при findings перезаписывает `review-findings.md` с confirmed findings (только актуальные проблемы текущей задачи, без task ID), записывает уроки в LEARNINGS.md и обновляет секцию ralph в CLAUDE.md, но НЕ вносит изменения в код. Каждый finding должен содержать достаточно информации чтобы следующая execute-сессия без дополнительного контекста могла понять: ЧТО не так, ГДЕ в коде, ПОЧЕМУ это проблема и КАК предлагается исправить
- **FR18:** При наличии findings система запускает следующую execute-сессию (тот же тип сессии — ralph не различает "первый execute" и "fix"). Execute видит непустой review-findings.md, адресует findings, запускает тесты, коммитит при green
- **FR18a:** После execute система запускает повторный review для верификации фиксов (цикл execute→review, до максимума `max_review_iterations` итераций, default 3)
- **FR19 (Growth):** При batch-ревью (`--review-every N`) система предоставляет аннотированный diff с маппингом TASK→AC→тесты

### Контроль качества (Gates)

- **FR20:** Разработчик может включить human gates через CLI-флаг
- **FR21:** Система останавливается на размеченных точках human gate для ввода разработчика
- **FR22:** Разработчик может одобрить, повторить с обратной связью, пропустить или выйти на gate. При retry с feedback ralph программно добавляет feedback в sprint-tasks.md под текущей задачей (индентированная строка `> USER FEEDBACK: ...`). Следующий execute читает sprint-tasks.md и видит feedback
- **FR23:** Система вызывает экстренный human gate когда AI исчерпал максимум попыток execute
- **FR24:** Система вызывает экстренный human gate когда цикл execute→review превысил максимум итераций
- **FR25:** Разработчик может установить периодические checkpoint gates каждые N задач

### Управление знаниями

- **FR26:** Система пишет операционные знания в секцию `## Ralph operational context` файла CLAUDE.md. Обновление — через review-сессию (при findings) и resume-extraction (при неуспехе execute): добавить новое, переформулировать для краткости, убрать дублирование (по модели Farr Playbook). Существующий контент проекта вне секции ralph не затрагивается
- **FR27:** Система записывает паттерны и выводы в LEARNINGS.md
- **FR28:** При неудачном execute (нет коммита) система возобновляет execute-сессию через `claude --resume` (resume-extraction). Execute-сессия имеет полный контекст — знает что пыталась, где застряла. Resume-extraction: (1) коммитит текущее WIP-состояние, (2) пишет прогресс под текущей задачей в sprint-tasks.md, (3) записывает причины неудачи и извлечённые знания в LEARNINGS.md + обновляет секцию ralph в CLAUDE.md
- **FR28a:** Review-сессия при наличии findings сама записывает уроки в LEARNINGS.md (какие типы ошибок, что агент забывает, паттерны для будущих сессий) и обновляет секцию ralph в CLAUDE.md — без отдельной extraction-сессии. Review имеет полный контекст анализа. При clean review — ставит `[x]`, очищает review-findings.md. После clean review ralph проверяет размер LEARNINGS.md и при превышении бюджета запускает отдельную distillation-сессию (`claude -p`). Если первый review сразу clean и LEARNINGS.md в пределах бюджета — distillation не нужна
- **FR28b:** Разработчик может включить флаг `--always-extract` для запуска resume-extraction после КАЖДОГО execute, включая успешные. Resume-extraction возобновляет execute-сессию и извлекает знания из **процесса выполнения** (какие решения принял Claude, что пошло хорошо, какие подходы сработали). Больше знаний, но дороже по токенам (дополнительный `claude --resume` на каждую задачу)
- **FR29:** Файлы знаний загружаются в контекст каждой новой сессии

### Конфигурация и кастомизация

- **FR30:** Разработчик может настраивать поведение через config-файл в корне проекта
- **FR31:** Разработчик может переопределять настройки config-файла через CLI-флаги
- **FR32:** Разработчик может кастомизировать промпты review-агентов через текстовые файлы
- **FR33:** Система использует fallback-цепочку: проектные → глобальные → встроенные конфигурации агентов
- **FR34:** Разработчик может настроить модель Claude для каждого review-агента
- **FR35:** Система возвращает информативные exit-коды для интеграции со скриптами

### Guardrails и ATDD

- **FR36:** Система применяет 999-series guardrail-правила в execute-промпте. 999-правила — последний барьер: даже если review-findings.md предлагает опасное действие, execute откажется
- **FR37:** Система обеспечивает ATDD: каждый acceptance criterion должен иметь соответствующий тест
- **FR38:** Система никогда не пропускает тесты — unit на каждый execute, e2e на UI-задачах и review-every checkpoint-ах. Падения исправляются или эскалируются
- **FR39:** Система обнаруживает наличие Serena MCP и использует его для чтения кода. Двойная ценность: (1) token economy в execute — semantic code retrieval вместо чтения целых файлов; (2) review accuracy — sub-агенты проверяют related code и интерфейсы, снижая false positives. При старте `ralph run` — полная индексация проекта (Serena full index, timeout 60s). Перед каждой execute-сессией — incremental index (timeout configurable, default 10s). При таймауте или недоступности Serena — graceful fallback на стандартное чтение файлов с progress output
- **FR40 (Growth):** При старте `ralph run` система проверяет версию Claude CLI (`claude --version`) и предупреждает при несовместимой или неизвестной версии. Все вызовы CLI абстрагированы через session adapter для поддержки будущих LLM-провайдеров
- **FR41 (Growth):** Перед запуском каждой execute-сессии система подсчитывает примерный размер контекста (промпт + контекстные файлы: CLAUDE.md секция, LEARNINGS.md, task description). При превышении порога 40% от context window — warning с рекомендацией сократить контекстные файлы
