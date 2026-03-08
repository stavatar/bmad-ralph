# Bridge Performance Analysis — Оптимизация скорости

## Executive Summary

`ralph bridge` для крупного проекта (34 stories, ~1.3MB) генерирует 6+ batch'ей последовательно. Merge mode делает параллелизацию невозможной без переделки. Максимальный ROI: увеличение batch size (2x) + preflight skip (10-19x при повторных запусках).

## Текущая архитектура

**Цепочка:** `runBridge` → `splitBySize(files, 80KB)` → цикл по batch'ам → `bridge.Run()` → `session.Execute()` (Claude CLI)

**Ключевые числа:**
- `maxBatchBytes = 80000` (~80KB)
- Overhead промпта: ~16.5KB (bridge.md 13KB + format contract 3.4KB)
- Batch 2+ работают в **merge mode** — промпт растёт на размер уже сгенерированного sprint-tasks.md
- Merge mode каскадный: batch N читает результат batch N-1, batch'и строго последовательные

**Критическое узкое место:** merge mode делает параллелизацию batch'ей невозможной без переделки архитектуры.

## Таблица подходов (отсортировано по ROI)

| Подход | Сложность | Ускорение | Описание | Риски |
|--------|-----------|-----------|----------|-------|
| **Увеличить maxBatchBytes до 160KB** | S | 2x | Одна строка: `maxBatchBytes = 160000`. Claude context 200K tokens; 160KB stories = ~45K tokens + ~16KB overhead = ~50K из 200K. Запас достаточный | Крупные batch'и могут давать менее точные результаты |
| **Preflight skip: пропуск обработанных stories** | S | 1.5-5x (повторные запуски) | Перед `splitBySize`: прочитать sprint-tasks.md, извлечь source поля, исключить stories с полным покрытием. `SourceFieldRegex` уже есть | Не помогает при первом запуске. Обновлённые stories будут пропущены |
| **Prompt trimming: сокращение bridge.md** | S | 1.1-1.2x | bridge.md 13KB содержит 6 примеров (Correct/WRONG). Для batch 2+ примеры не нужны → `bridgePromptShort` ~5KB | Риск деградации качества. Экономия ~8KB на batch |
| **Model selection (Haiku для простых)** | M | 2-3x для простых | Эвристика: stories < 5KB с ≤ 2 AC → Sonnet/Haiku. `session.Options.Model` уже поддержан | Нужна эвристика "простоты". Haiku может не соблюдать format contract |
| **Параллелизация batch'ей** | L | 3-5x | Все batch'и параллельно без merge, Go-код склеивает по epic headers | Сложная семантика склейки. Дупликаты. Rate limiting |
| **Claude API Message Batches (async)** | L | 5-10x | Batch API: все запросы одним вызовом, 50% скидка на стоимость | Нужен API key вместо CLI. Ломает session layer. До 24ч ожидания |
| **Prompt caching через API** | M | 1.5-2x | `cache_control` breakpoint после system prompt. 90% скидка на cached portion | Требует API вместо CLI. Cached portion мала (16KB из 100KB+) |
| **Incremental generation (1 story = 1 вызов)** | M | 0.5-2x | По одному story за раз. Маленький prompt (~30KB), но 85 вызовов вместо 19 | При 85 stories скорее **медленнее** из-за overhead на запуск CLI |
| **Two-pass: классификация + генерация** | L | 0.5x (медленнее!) | Два вызова вместо одного. Классификация AC — простая часть | Увеличивает количество вызовов без реальной выгоды |
| **Pre-parsed story metadata** | M | 1.1x | Парсить AC регулярками, передавать JSON. Уменьшает tokens на 30-40% | Хрупкий парсинг. Потеря контекста |
| **Local generation (без Claude)** | L | 10x для тривиальных | Для stories с 1-2 AC генерировать задачи Go-кодом | Очень ограниченная применимость. Потеря качества |
| **Кэширование parsed template** | S | 1.05x | Кэшировать `template.Parse()` между batch'ами | Нулевой эффект. Bottleneck — вызов Claude, не сборка промпта |

## Рекомендации (в порядке внедрения)

### Этап 1 — Quick wins (1-2 часа)

1. **Увеличить `maxBatchBytes` до 160000** — одна строка в `cmd/ralph/bridge.go:27`. Сокращает batch'и вдвое. Ускорение ~2x.

2. **Preflight skip** — перед `splitBySize`:
```go
files = filterNewStories(cfg.ProjectRoot, files)
```
При повторных запусках (типичный use case — 2-3 новых stories) сокращает работу с 19 batch'ей до 1.

### Этап 2 — Средняя сложность (4-8 часов)

3. **Compact prompt для merge batch'ей** — для batch 2+ использовать сокращённый промпт без примеров. Экономия ~8KB на batch.

4. **Model selection** — stories < 5KB с ≤ 2 AC → Sonnet (быстрее и дешевле).

### Этап 3 — Архитектурные (дни)

5. **Параллелизация без merge** — все batch'и параллельно, Go-код склеивает результаты.

### Главный вывод

Комбинация 1+2 даёт **максимальный ROI**: повторные запуски ускоряются в **10-19x** при минимальной сложности (S+S). Первый запуск — 2x от увеличения batch.

## Источники

- [Claude API Batch Processing](https://platform.claude.com/docs/en/build-with-claude/batch-processing)
- [Prompt Caching в Claude API](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)
- [Prompt Caching в Claude Code](https://www.claudecodecamp.com/p/how-prompt-caching-actually-works-in-claude-code)
- [LLM Parallel Processing Best Practices](https://dev.to/jamesli/llm-parallel-processing-in-practice-key-techniques-for-performance-enhancement-20g0)
- [Batch Processing Optimization для LLM](https://latitude-blog.ghost.io/blog/how-to-optimize-batch-processing-for-llms/)
