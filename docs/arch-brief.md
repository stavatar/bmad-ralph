# Architecture Brief — ralph v2

## Входные документы
- `docs/prd.md` — FR1-FR29, NFR1-NFR19, CLI Tool Specific Requirements
- `docs/project-context.md` — текущая архитектура ralph v1

## Ключевые решения для проработки

1. **Новый пакет `plan/`** — структура, интерфейсы, dependency direction (`cmd/ralph → plan → config, session`)
2. **Typed `PlanInput` struct** — `{File string, Role string}` для передачи в LLM
3. **Auto review** — reviewer запускается в отдельной Claude сессии (чистый контекст)
4. **Merge mode** — алгоритм обнаружения существующих stories и добавления новых задач без дублирования
5. **Typed headers** — формат `<!-- file: <name> | role: <role> -->` в промпте LLM
6. **Bridge removal** — что удаляется, что остаётся, как избежать регрессий
7. **Промпт архитектура** — `plan/prompts/plan.md` (generator) отделён от reviewer промпта

## Ограничения

- Dependency direction строго top-down: `cmd/ralph → plan → config, session` — циклы запрещены
- `plan` НЕ зависит от `bridge` (bridge удаляется)
- Только 3 внешних dep: cobra, yaml.v3, fatih/color — новые требуют обоснования
- Single binary, CGO_ENABLED=0
- Exit codes ONLY в `cmd/ralph/`, пакеты возвращают errors, никогда os.Exit
