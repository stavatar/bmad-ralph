# Research 3: E2E тесты в Ralph Loop

**Дата:** 2026-02-24
**Вопрос:** Насколько активно использовать browser E2E в Ralph цикле? Когда запускать?

---

## 1. Проблема

E2E тесты (Playwright/Cypress) дорогие: 10-60 секунд на тест, требуют запущенное приложение, хрупкие. В Ralph loop каждая итерация заканчивается тестами. Запускать E2E каждую итерацию = замедление цикла. Не запускать = пропуск regression.

## 2. Как тестируют в Ralph-реализациях

### 2.1 Clayton Farr Playbook
- Тесты — обязательные completion criteria
- Агент обязан написать и пройти тесты перед коммитом
- **Не разделяет** unit vs E2E — "required tests" определены в implementation plan
- Для субъективных критериев → LLM-as-Judge

### 2.2 ralphex
- Validation commands (тесты, линтеры) запускаются **после каждой задачи**
- Нет специфики по E2E vs unit — конфигурируется пользователем
- Четырёхфазная система: execute → review → codex review → final review

### 2.3 Общий паттерн в сообществе
- **Unit тесты:** каждая итерация (быстрые, дешёвые, детерминированные)
- **E2E тесты:** нет чёткого консенсуса по частоте
- **AI-powered E2E:** Playwright MCP, TestSprite, Shortest — можно генерировать E2E тесты через AI

## 3. Стратегии запуска E2E

### Стратегия A: E2E каждую итерацию

```bash
# loop.sh — после каждого task
vitest run          # unit + integration
playwright test     # E2E
```

**Плюсы:** Максимальный backpressure, regression сразу ловится
**Минусы:** +30-120 сек на итерацию, хрупкость E2E → ложные fail → retry

### Стратегия B: E2E периодически (каждые N итераций)

```bash
# loop.sh
ITER_COUNT=$((ITER_COUNT + 1))
vitest run                                    # unit — всегда
if [ $((ITER_COUNT % 5)) -eq 0 ]; then
  playwright test                             # E2E — каждые 5 итераций
fi
```

**Плюсы:** Баланс скорости и качества
**Минусы:** Regression может накопиться за 5 итераций

### Стратегия C: E2E по типу задачи

```markdown
# sprint-tasks.md
- [ ] TASK-1: Add login API [tests: unit]
- [ ] TASK-2: Create login page [tests: unit, e2e]
- [ ] TASK-3: Refactor auth utils [tests: unit]
- [ ] TASK-4: Full auth flow [tests: unit, e2e, smoke]
```

**Плюсы:** E2E только когда релевантно (UI задачи), экономия
**Минусы:** Нужно размечать задачи, код-конвертер должен знать типы

### Стратегия D: Tiered testing (рекомендуемая)

| Уровень | Когда | Что | Время |
|---------|-------|-----|-------|
| **Fast** | Каждая итерация | `vitest run` (unit + integration) | 5-15 сек |
| **Medium** | Задачи с UI | `playwright test --grep @smoke` | 15-30 сек |
| **Full** | Каждые N итераций или SERVICE task | `playwright test` (полный suite) | 1-3 мин |
| **Regression** | Конец спринта / epic | Полный E2E + visual regression | 5-10 мин |

## 4. AI-powered E2E: новые возможности (2025-2026)

### Playwright MCP
- AI агент может взаимодействовать с живым браузером через MCP
- Генерация E2E тестов из описания на естественном языке
- Интеграция с Claude Code / Copilot

### TestSprite
- AI поднимает pass rate от 42% до 93% за одну итерацию
- Автоматический planning → testing → debugging loop

### Практические метрики
- AI-powered создание тестов: -80% времени
- Flaky tests reduction: -85%
- Coverage growth: 380 → 700+ tests (один кейс)

## 5. Рекомендация для bmad-ralph

### MVP: Стратегия C (по типу задачи) + Fast tier

1. **Код-конвертер** при генерации задач определяет `tests: unit` или `tests: unit, e2e` на основе story контекста
2. **loop.sh** после каждой итерации запускает:
   - `vitest run` — всегда (fast, unit/integration)
   - `playwright test --grep @smoke` — только если задача помечена `e2e`
3. **SERVICE-задача** "Full E2E suite" — добавляется автоматически каждые 5-10 задач

### Production: Стратегия D (Tiered) + Playwright MCP

- Fast tier: каждая итерация
- Medium tier: UI-задачи
- Full tier: SERVICE-задача или human gate
- Playwright MCP для генерации E2E тестов агентом

### Формат в sprint-tasks.md

```markdown
- [ ] TASK-3: Implement login page
  tests: unit, e2e
  e2e-scope: @auth @smoke
```

## Источники

- [Playwright MCP integration](https://developer.microsoft.com/blog/the-complete-playwright-end-to-end-story-tools-ai-and-real-world-workflows)
- [Anthropic — Demystifying evals](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- [700+ tests case study (OpenObserve)](https://openobserve.ai/blog/autonomous-qa-testing-ai-agents-claude-code/)
- [ralphex — test strategy](https://ralphex.com/)
- [Clayton Farr Ralph Playbook](https://claytonfarr.github.io/ralph-playbook/)
