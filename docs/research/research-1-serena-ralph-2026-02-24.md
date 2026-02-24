# Research 1: Serena + Ralph Loop Integration

**Дата:** 2026-02-24
**Вопрос:** Как интегрировать Serena (code indexing) с философией Ralph цикла?

---

## 1. Проблема

Serena MCP даёт агенту semantic code retrieval (find_symbol, find_referencing_symbols) вместо чтения всех файлов — экономия токенов. Но Ralph loop = fresh context каждую итерацию. Как поддерживать индекс актуальным между итерациями?

## 2. Как работает индексирование Serena

- **Начальный индекс:** `serena project index` создаёт symbol cache (`.serena/cache/`)
- **Инкрементальный:** `serena project index --incremental` — только изменённые файлы
- **Watch mode:** `serena project watch` — автоматическое обновление при изменении файлов
- **Git hooks:** `post-checkout` → `serena project index --incremental`
- **Параллельность:** `--parallel N` (50-75% CPU cores)

### Когда обновлять (рекомендации SmartScope):

| Событие | Тип индексации |
|---------|---------------|
| Первый запуск | Full index |
| Переключение ветки | Incremental |
| Редактирование файла | Watch mode |
| После merge | Incremental |
| После npm install / dep update | Full index |
| Еженедельно / при stale | Full (`--force-full`) |

### Оптимизация для больших проектов:

- Исключить `node_modules`, `dist`, `build`, `.git` → 66-71% сокращение файлов
- Фильтр по языкам (только TS/JS/Python)
- Dashboard: `localhost:24282/dashboard/index.html`

## 3. Стратегии интеграции с Ralph Loop

### Стратегия A: Reindex как служебная задача (рекомендуемая)

```markdown
# sprint-tasks.md — код-конвертер добавляет автоматически
- [ ] SERVICE: Reindex project (Serena)
- [ ] TASK-1: Implement auth module
- [ ] SERVICE: Reindex project (Serena)
- [ ] TASK-2: Implement user profile
```

**Плюсы:** Полностью Ralph-compatible, гарантированная актуальность
**Минусы:** Дополнительная итерация на каждый reindex

### Стратегия B: Bash hook в loop.sh (рекомендуемая)

```bash
# loop.sh — reindex ПЕРЕД каждой итерацией
while true; do
  serena project index --incremental
  claude -p "..." --system "$(cat PROMPT_build.md)"
  # check completion...
done
```

**Плюсы:** Нулевые расходы ходов агента, автоматически, быстро (incremental)
**Минусы:** Внешняя зависимость (serena CLI должна быть установлена)

### Стратегия C: Watch mode (фон)

```bash
# Запустить watch перед loop
serena project watch &
WATCH_PID=$!

# Ralph loop
while true; do
  claude -p "..."
done

kill $WATCH_PID
```

**Плюсы:** Полностью прозрачно, индекс всегда актуален
**Минусы:** Потребляет CPU, может конфликтовать с быстрыми итерациями

## 4. Рекомендация для bmad-ralph

### MVP: Full index на старте + incremental в loop

```bash
# Перед стартом loop — полный индекс:
serena project index --parallel 4 2>/dev/null || true

# В loop перед каждой итерацией — incremental:
serena project index --incremental 2>/dev/null || true
```

- Full index на старте — гарантирует полную картину кодовой базы
- Incremental в loop — 2-5 секунд, только изменённые файлы
- Не тратит ходы агента
- Fallback: если Serena не установлена — пропуск (`|| true`)

## 5. Staleness не проблема

Ключевой инсайт: в Ralph loop **staleness решается архитектурно**:

1. Каждая итерация = один коммит (обычно)
2. Incremental reindex перед следующей итерацией = все изменения учтены
3. Agent не видит stale данных — он начинает с fresh context + fresh index

Это лучше, чем в обычном длинном сеансе, где индекс устаревает по мере работы.

## Источники

- [Serena GitHub](https://github.com/oraios/serena)
- [Serena Indexing Optimization (SmartScope)](https://smartscope.blog/en/ai-development/serena-mcp-project-indexing-optimization/)
- [Serena MCP Setup Guide (SmartScope)](https://smartscope.blog/en/generative-ai/claude/serena-mcp-implementation-guide/)
- [Serena на ThoughtWorks Radar](https://www.thoughtworks.com/radar/tools/serena)
