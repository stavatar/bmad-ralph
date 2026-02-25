# Project Scoping & Risk

### MVP Strategy

**Approach: Problem-Solving MVP** — решить core problem (ручная оркестрация AI = 30-50% overhead) минимальным набором features.

**MVP = 7 компонентов:**
1. `ralph bridge` — story → tasks с AC-derived tests и human gates
2. `ralph run` — loop, execute → review (свежие сессии)
3. 4 review sub-агента — quality, implementation, simplification, test-coverage
4. Human gates — approve, retry, skip, quit + emergency gate
5. Knowledge extraction — LEARNINGS.md + CLAUDE.md
6. Guardrails — 999-series правила в execute-промпте
7. Configuration system — `.ralph/config` + CLI flags override, agent files fallback chain

**Aha-test:** Запустил `ralph run` — 10 задач выполнились автономно с quality review. Без ручного кормления контекста.

### Risk Mitigation

| Риск | Вероятность | Импакт | Митигация |
|------|:-----------:|:------:|-----------|
| **Orchestration complexity** — оркестрация 3 типов сессий, config parsing, human gates, subprocess management | Средняя | Средний | Go single binary: встроенный парсинг, типизация, тесты |
| **Нишевая аудитория** — нужны и BMad, и Ralph Loop | Средняя | Высокий | Quick start (`ralph run --plan`) в Growth снижает порог входа |
| **Claude Code API changes** — CLI flags, Task tool могут измениться | Средняя | Высокий | Абстракция вызовов через config (`claude_command`). Мониторинг changelog |
| **Review quality** — false positives от sub-агентов | Средняя | Средний | Верификация находок перед записью в findings. Настраиваемые agent models |
| **Solo developer** | Высокая | Средний | Lean MVP. Open source с первого дня |
