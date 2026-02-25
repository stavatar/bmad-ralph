# Product Scope

### MVP - Minimum Viable Product

1. **`ralph bridge`** — детерминированный конвертер BMad stories → `sprint-tasks.md` с AC-derived test cases, human gates, служебными задачами. Smart Merge при повторном запуске.
2. **`ralph run`** — loop с fresh context (Go single binary). Review после каждой задачи. Serena integration (best effort). ATDD-lite. Режимы: без gates, `--gates`, `--gates --every N`.
3. **4 параллельных review sub-агента** — quality, implementation, simplification, test-coverage. Critical finding = итерация не пройдена. Review сама записывает findings-знания.
4. **Human gates** — approve, retry (feedback → fix-задача), skip, quit. Экстренный human gate при застревании AI.
5. **Knowledge extraction (три механизма)** — (1) execute пишет learnings перед завершением (best effort), (2) review записывает findings-знания при анализе, (3) resume-extraction (`claude --resume`) при неуспехе execute — коммит WIP + прогресс + знания. Дистилляция LEARNINGS.md с hard limit при превышении бюджета.
6. **Guardrails** — 999-series правила из Farr Playbook в execute-промпте.

### MVP Phase 2 (после стабилизации основного loop)

1. **Correct flow** (c на human gate) — правка BMad story → автоматический re-bridge. Требует stable bridge + run + human gates.
2. **Circuit breaker** — автоматическая остановка при серии неуспехов (CLOSED/HALF_OPEN/OPEN).
3. **Lightweight review** — при малом diff (<50 строк) и быстром execute (<5 ходов) — упрощённый review (1 агент вместо 4). Фокус на проверке AC из story-файла.

### Growth Features (Post-MVP)

- Rollback retry: git reset + повтор задачи с feedback
- LLM-as-Judge test fixtures: формальный quality gate через `claude -p`
- Quick start: `ralph run --plan plan.md` для пользователей без BMad
- Notifications: desktop/Slack при human gate, завершении спринта
- Performance и security review agents
- Custom skills из LEARNINGS.md: автоматическое создание переиспользуемых навыков из повторяющихся паттернов (level 3 knowledge extraction)
- Cross-model review: внешний reviewer (Codex, другая модель) для независимой проверки
- Hook-based review: review через Claude Code hooks как альтернатива sub-agent подходу
- Vision-based LLM-as-Judge: скриншот-сравнение UI с макетом для автоматической проверки визуала
- "Fix the neighborhood" (Farr): автоматическое исправление не связанных падающих тестов при обнаружении
- Context budget calculator: подсчёт размера контекста (промпт + файлы) перед сессией, warning при >40% context window
- CLI version check: проверка совместимости версии Claude CLI при старте, compatibility matrix
- Review severity filtering: `review_min_severity` в конфиге, findings ниже порога не блокируют pipeline
- Session adapter (multi-LLM): абстракция вызовов для поддержки Gemini/других LLM-провайдеров
- Structured log format: JSON/tab-separated лог для автоматического анализа и метрик
- goreleaser: автоматизированная сборка и публикация binary через GitHub Releases
- Batch review (`--review-every N`): review после каждых N задач с аннотированным diff и маппингом TASK→AC→tests
- Smart resume: при crash recovery — если последний коммит соответствует текущей задаче, skip execute и сразу review (оптимизация лишнего API-call)
- Contributor guide: документация для контрибьюторов (setup, architecture overview, PR process)
- Safe extraction order: extraction → commit → очистка review-findings.md. При падении extraction — findings не очищаются, но задача не блокируется
- Resilient resume-extraction: retry resume-extraction при сбое, partial commit recovery

### Vision (Future)

- Полная BMad CLI интеграция: `ralph init` → `ralph plan` → `ralph bridge` → `ralph run`
- Multi-agent parallelism: несколько задач одновременно на разных ветках
- Team mode: несколько разработчиков, общий sprint-tasks.md
- Plugin system: кастомные review agents, bridge adapters, notification providers
- Dashboard с метриками спринта
- Интеграция с другими planning-фреймворками
