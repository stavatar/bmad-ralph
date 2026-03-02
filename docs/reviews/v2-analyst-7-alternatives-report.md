# Analyst-7: Alternative Knowledge Management Approaches for Ralph

**Дата:** 2026-03-02
**Контекст:** Epic 6 v5 review, team knowledge-arch-v2
**Базовые исследования:** R1 (extraction), R2 (enforcement), R3 (alternatives)

---

## Executive Summary

Исследованы 8 альтернативных подходов к управлению знаниями для Ralph. **Текущий план Epic 6 (extract -> categorize -> distill -> inject) остаётся оптимальным** для ограничений проекта: single-process Go, минимум зависимостей, `claude --print` pipe mode, работа на новых проектах с нуля.

**Критическое открытие:** `claude -p` mode с `--allowedTools` поддерживает tools и hooks. Без `--allowedTools` — tools недоступны, hooks не fire-ят. Ralph уже использует `--allowedTools` через `session.Execute`, поэтому hooks-based подходы **работают** в текущей архитектуре.

**Ключевые выводы:**
1. Простой append (подход 1) — жизнеспособен до ~200 строк, но без distillation деградирует
2. MCP tool (подход 2) — блокирован 56% skip rate без forced invocation
3. Hooks-based (подход 3) — УЖЕ реализован в bmad-ralph, validated, расширяем
4. Lazy loading (подход 4) — рискован в pipe mode, Claude может не "решить" прочитать
5. BM25/keyword (подход 5) — разумен при >500 правил, преждевременен сейчас
6. Embeddings/RAG (подход 6) — невозможен без внешних зависимостей (embedding API)
7. Knowledge compilation (подход 7) — ломает модель: Ralph не знает правил целевого проекта
8. Ничего не делать (подход 8) — auto-memory ненадёжен для autonomous agent

---

## Методология

- 3 предшествующих исследования (R1: 20 источников, R2: 40 источников, R3: 22 источника)
- Web-исследование Claude Code `--print` mode capabilities (hooks, MCP, tools)
- Анализ текущей архитектуры Ralph (runner/knowledge.go, session.Execute, .claude/settings.json)
- Оценка каждого подхода по 5 критериям

---

## Подход 1: Простой Append (всё в одном файле)

### Описание
Все извлечённые знания записываются append-only в один файл (LEARNINGS.md). Никакой обработки, категоризации, дистилляции. Файл загружается целиком в каждый промпт.

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~50-100 LoC. Минимальная: `os.ReadFile` + `os.WriteFile` с append. 0 зависимостей |
| **Новый проект** | Да — файл создаётся при первом запуске |
| **`claude --print`** | Да — файл читается Go-кодом, инжектируется в промпт |
| **Масштабируемость** | 10 правил: отлично. 100: хорошо. 200+: context rot [R1-S5]. 500+: неработоспособно |
| **Риски** | (1) Неограниченный рост → context rot. (2) Нет dedup → повторы. (3) Нет validation → мусорные записи. (4) Нет приоритизации — все правила равноценны |

### Вердикт
**Базовый уровень, от которого отталкивается Epic 6.** Работает для маленьких баз (<200 строк), но без distillation и validation деградирует. Epic 6 правильно добавляет post-validation (6.1), distillation (6.2-6.3) и категоризацию (6.4) поверх этого базового подхода.

---

## Подход 2: MCP Tool (знания как MCP-инструмент)

### Описание
Знания хранятся в MCP Memory Server (knowledge graph: entities → relations → observations, JSONL). Claude сам вызывает MCP tools (`search_nodes`, `read_graph`) когда считает нужным.

### Реализация
- Официальный `@modelcontextprotocol/server-memory` — Node.js [R3-S4]
- Go-based альтернатива: `github.com/mark3labs/mcp-go` (sidecar process)
- 9 CRUD tools: create/delete entities, relations, observations + search + read

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~50 LoC config (если Node.js). ~500-800 LoC если Go MCP server. +1 зависимость (mcp-go) |
| **Новый проект** | Условно — нужен настроенный MCP server. Ralph должен генерировать `.mcp.json` |
| **`claude --print`** | **КРИТИЧЕСКОЕ ОГРАНИЧЕНИЕ:** MCP tools доступны в `-p` mode только с `--allowedTools`. Нужно явно разрешить `mcp__memory__*` tools. Работоспособно, но требует дополнительной конфигурации |
| **Масштабируемость** | До ~10K entries. Knowledge graph хорошо масштабируется |
| **Риски** | (1) **56% skip rate** — Claude пропускает MCP tools в >50% случаев без forced invocation [R2-S31]. (2) Node.js runtime зависимость. (3) Нет гарантии pre-loading — Claude может не запросить нужные знания. (4) Latency: 50-200ms на каждый tool call |

### Критический анализ: skip rate
Исследования показывают 56% skip rate для MCP tools, когда агент сам решает их вызывать [R2-S31]. Это означает, что в autonomous mode (ralph execute) половина знаний будет проигнорирована. Для исправления нужен forced invocation через hooks — но тогда hooks-based injection проще напрямую.

### Вердикт
**Не рекомендуется для текущего масштаба.** MCP добавляет runtime зависимость (Node.js или Go sidecar), а skip rate делает retrieval ненадёжным. Для Growth phase (>500 entries, multi-project) — пересмотреть при условии forced invocation через hooks.

---

## Подход 3: Hooks-Based (знания через Claude Code hooks)

### Описание
Знания инжектируются через Claude Code hooks:
- **SessionStart** — критические правила при старте сессии
- **PreToolUse** — контекстные чеклисты перед Edit/Write
- **PostToolUse** — автоматические фиксы (CRLF)

### Текущее состояние в bmad-ralph
УЖЕ реализовано в `.claude/settings.json`:
- SessionStart → `cat .claude/critical-rules.md`
- PreToolUse (Edit|Write) → `pre-edit-checklist.sh`
- PostToolUse (Edit|Write) → `fix-crlf.sh`

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~100-200 LoC (shell scripts + config). Уже реализовано в проекте |
| **Новый проект** | **Ключевое ограничение:** hooks в `.claude/settings.json` — user-level config. Ralph НЕ МОЖЕТ программно создать hooks для целевого проекта пользователя. Hooks работают для САМОГО Ralph (его внутренние сессии), но не для проектов которые Ralph оркестрирует |
| **`claude --print`** | **Работает** с `--allowedTools` — hooks fire при tool calls. SessionStart fires при старте сессии. Подтверждено документацией: `-p` mode = headless mode, CLI options work the same way |
| **Масштабируемость** | SessionStart: ~15 правил (94% compliance [R2-S25]). PreToolUse: ~5-7 items per checklist. PostToolUse: неограниченно (deterministic scripts). Суммарно: ~50 активных правил через hooks |
| **Риски** | (1) Hook overhead: >500 tokens/turn → context dilution [R2-5.1]. (2) Checklist fatigue при >10 items. (3) Hooks НЕ переносимы на целевой проект пользователя |

### Ключевое уточнение: scope hooks
Ralph оркестрирует Claude Code сессии на ЦЕЛЕВОМ проекте пользователя. Hooks из `.claude/settings.json` Ralph-а применяются к ТОМ ЖЕ CLI invocation. Поскольку Ralph использует `session.Execute` (which calls `claude -p`), hooks fire в контексте вызова.

**Но:** Ralph мог бы ГЕНЕРИРОВАТЬ `.claude/settings.json` для целевого проекта. Это архитектурно рискованно (CVE-2025-59536, CVE-2026-21852 — программное редактирование config = confirmed risk class).

### Вердикт
**Уже реализовано и validated.** Наиболее эффективный enforcement mechanism для критических правил. Ограничение: ~50 правил через hooks. Для остального — file-based injection (Epic 6 plan). Расширение: Ralph может генерировать project-specific hooks в `.claude/settings.json` целевого проекта, но это высокорискованно.

---

## Подход 4: Lazy Loading (оглавление + Claude сам читает)

### Описание
Ralph инжектирует в промпт только TOC (оглавление) знаний. Claude сам решает какой файл прочитать через Read tool, загружая только релевантные знания.

### Реализация
```
## Knowledge Index
- testing-errors.md (15 rules about error testing patterns)
- testing-assertions.md (23 rules about assertion patterns)
- code-quality.md (28 rules about production code quality)
- wsl-ntfs.md (15 rules about WSL/NTFS specifics)
Read the relevant file when working on matching tasks.
```

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~30-50 LoC. Генерация TOC + размещение файлов. 0 зависимостей |
| **Новый проект** | Да — файлы создаются Ralph-ом |
| **`claude --print`** | **Только с `--allowedTools "Read"`** — Claude должен иметь право вызвать Read tool. Работает, но добавляет tool calls |
| **Масштабируемость** | Отлично: TOC = ~10-20 строк, файлы неограниченны. Теоретически до 1000+ правил |
| **Риски** | (1) **Claude может не прочитать нужный файл** — та же проблема что с MCP (agent decides). (2) Extra tool calls = latency + token cost. (3) В `--max-turns` limited mode каждый Read — потерянный turn. (4) Нет гарантии что Claude прочитает ВСЕ нужные файлы |

### Сравнение с .claude/rules/
Claude Code УЖЕНАТИВНО реализует lazy loading через `.claude/rules/` с glob patterns! Файлы загружаются автоматически при работе с matching files. Это ЛУЧШЕ чем manual lazy loading, потому что:
- Автоматическое, не зависит от решения Claude
- Не тратит tool calls / turns
- Glob patterns = точное таргетирование

### Вердикт
**Нативный `.claude/rules/` с glob patterns лучше ручного lazy loading.** Epic 6 уже использует `.claude/rules/ralph-{category}.md` для distilled знаний (Tier 2). Ручной lazy loading добавляет сложность без преимущества.

---

## Подход 5: BM25/Keyword Search

### Описание
Ralph анализирует текст текущей задачи (story description, файлы в scope) и находит релевантные правила через keyword matching (BM25, TF-IDF, или простой text search).

### Реализация
```go
// Pseudo-code
func FindRelevantRules(taskText string, rules []Rule) []Rule {
    tokens := tokenize(taskText)
    scores := bm25Score(tokens, rules)
    return topK(scores, 20)
}
```

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~300-500 LoC. BM25 в чистом Go — возможно без зависимостей (stdlib math). Tokenizer: `strings.Fields` + stopwords |
| **Новый проект** | Да — правила создаются Ralph-ом, поиск работает сразу |
| **`claude --print`** | Да — поиск выполняется Go-кодом ДО вызова Claude. Результаты инжектируются в промпт |
| **Масштабируемость** | Хорошо: BM25 работает на 10K+ документов за <10ms |
| **Риски** | (1) **False negatives** — keyword mismatch пропускает релевантные правила. "Doc comments" vs "documentation accuracy" — не match. (2) Keyword search не понимает семантику: правило про error wrapping не найдётся при задаче "fix validation". (3) Требует quality index — правила должны содержать searchable keywords |

### Сравнение с Epic 6 pre-loading
При 200 строках LEARNINGS.md (~3000-4000 tokens) pre-loading эффективнее: загружаем всё, 0 false negatives, 0 latency. BM25 экономит tokens при >500 правил, но рискует пропустить важные правила.

### Вердикт
**Преждевременен для текущего масштаба (122-300 правил).** Разумная опция для Growth phase (500+ правил), когда pre-loading saturates context window. Может быть реализован без внешних зависимостей. Промежуточный вариант между full pre-load и RAG.

---

## Подход 6: Embeddings/RAG

### Описание
Правила индексируются через embedding модель, при каждой задаче выполняется vector similarity search для нахождения top-K релевантных правил.

### Возможность без зависимостей
- **chromem-go** — единственный pure-Go vector store (0 deps, CGO_ENABLED=0) [R3-S3]
- Performance: 0.5ms query на 1000 docs
- **НО:** требует внешний embedding API (OpenAI, Ollama, etc.)

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~500-800 LoC. +1 dep (chromem-go). +Embedding API (OpenAI key или Ollama server) |
| **Новый проект** | **Проблема:** нужен embedding API на машине пользователя. OpenAI = API key. Ollama = отдельная установка. Ни то ни другое не гарантировано |
| **`claude --print`** | Да — search выполняется Go-кодом до вызова Claude |
| **Масштабируемость** | Отлично: до 100K+ documents |
| **Риски** | (1) **Hard dependency на embedding API** — ломает принцип "работает с нуля". (2) Embedding latency: 100-300ms per query. (3) ~85-95% recall — не 100%. (4) +1 direct dep (нарушает "only 3 deps" constraint) |

### Pre-computed embeddings (альтернатива)
Можно предварительно вычислить embeddings при distillation (когда Claude доступен) и хранить в JSON. Cosine similarity в чистом Go = ~0.1ms для 500 entries. Но:
- Embeddings нужно пересчитывать при каждом изменении правил
- Embedding модель (какая?) — нестандартизировано
- 768-dim float32 × 500 entries = ~1.5MB файл

### Вердикт
**Невозможен без внешних зависимостей.** Добавляет hard dependency (embedding API) которая не гарантирована на новом проекте. При текущем масштабе (200-300 правил) pre-loading + distillation эффективнее. Отложить до момента когда (1) >1000 правил И (2) pre-loading доказано неэффективен.

---

## Подход 7: Knowledge Compilation (правила в Go-код)

### Описание
Правила компилируются в Go-код при build time через `go generate` или встраиваются через `//go:embed`. Поиск выполняется Go-кодом без runtime parsing.

### Реализация
```go
//go:embed rules/*.md
var rulesFS embed.FS

func GetRules(category string) []Rule {
    // compiled lookup table
}
```

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | ~200-400 LoC. `embed.FS` уже в stdlib. 0 зависимостей |
| **Новый проект** | **ФУНДАМЕНТАЛЬНАЯ ПРОБЛЕМА:** Ralph не знает правил целевого проекта на этапе компиляции! Знания извлекаются в RUNTIME из code reviews конкретного проекта. Compile-time embedding работает только для СОБСТВЕННЫХ правил Ralph-а (Go conventions, etc.) |
| **`claude --print`** | Да — embedded content читается Go-кодом |
| **Масштабируемость** | Для static rules: отлично. Для dynamic rules: неприменимо |
| **Риски** | (1) Невозможность для project-specific знаний. (2) Stale при обновлении — нужен rebuild |

### Что может быть compiled
- **Шаблоны промптов** (уже используются: `runner/prompts/*.md` через `embed.FS`)
- **Default правила** (Go best practices, общие patterns) — не меняются между проектами
- **Format templates** для LEARNINGS.md entries

### Вердикт
**Неприменим для project-specific знаний.** Ralph запускается на НОВОМ проекте с нуля — знания не существуют на момент компиляции. Compile-time embedding уже используется для промптов и default templates — дальнейшее расширение бесполезно.

---

## Подход 8: Ничего не делать (auto-memory Claude Code)

### Описание
Ralph не управляет знаниями. Claude Code сам учится через auto-memory (`~/.claude/MEMORY.md`), `.claude/rules/` загружаются нативно. Пользователь/агент сам решает что запомнить.

### Оценка

| Критерий | Оценка |
|----------|--------|
| **Сложность** | 0 LoC. Никакой реализации не нужно |
| **Новый проект** | Да — auto-memory работает из коробки |
| **`claude --print`** | **ПРОБЛЕМА:** в pipe mode MEMORY.md загружается, но auto-memory (запись) может не работать. Claude в `-p` mode не имеет interactive prompts для "save to memory" |
| **Масштабируемость** | 200 строк MEMORY.md — hard limit [R1-S4]. Topic files on demand |
| **Риски** | (1) **Claude сам решает что запомнить** — качество непредсказуемо. (2) В autonomous mode (ralph execute) Claude не получает feedback "это стоит запомнить". (3) Auto-memory = per-user, не per-project. (4) Нет structure: всё в произвольном формате. (5) Нет distillation — 200 строк заканчиваются быстро |

### Реальный опыт bmad-ralph
Текущий MEMORY.md bmad-ralph (40+ строк) написан вручную — не auto-memory. Auto-memory Claude Code хранит разрозненные заметки без структуры. Для autonomous agent, который должен учиться через code reviews, passive auto-memory недостаточен.

### Вердикт
**Недостаточен для autonomous agent.** Auto-memory = passive, per-user, unstructured, limited to 200 lines. Ralph нуждается в active, per-project, structured knowledge с distillation. "Ничего не делать" = потеря всех learnings между сессиями.

---

## Сводная таблица

| # | Подход | Сложность (LoC) | Новый проект | `claude --print` | Масштаб (10→1000) | Главный риск | **Оценка** |
|---|--------|-----------------|-------------|------------------|-------------------|-------------|------------|
| 1 | Simple append | ~50-100 | Да | Да | 10-200: ОК, 500+: нет | Context rot | 6/10 |
| 2 | MCP tool | ~50-800 | Условно | С --allowedTools | До 10K | 56% skip rate | 4/10 |
| 3 | Hooks-based | ~100-200 | Для Ralph: да | Да | ~50 правил через hooks | Hook overhead | **8/10** |
| 4 | Lazy loading | ~30-50 | Да | С Read tool | До 1000+ | Agent may not read | 5/10 |
| 5 | BM25/keyword | ~300-500 | Да | Да | До 10K+ | False negatives | 6/10 |
| 6 | Embeddings/RAG | ~500-800 | Нет (нужен API) | Да | До 100K+ | Hard dependency | 3/10 |
| 7 | Compilation | ~200-400 | Нет (static only) | Да | Static only | Dynamic rules impossible | 2/10 |
| 8 | Ничего не делать | 0 | Да | Частично | 200 строк max | Unstructured, passive | 3/10 |

---

## Рекомендация: Гибридный подход (что делает Epic 6)

Epic 6 фактически реализует **гибрид подходов 1 + 3 + 4(native)**:

| Компонент Epic 6 | Базовый подход | Дополнение |
|-------------------|---------------|------------|
| LEARNINGS.md append | Подход 1 (simple append) | + post-validation (6.1) + distillation (6.2-6.3) |
| `.claude/rules/ralph-{category}.md` | Подход 4 (lazy loading) | Нативный через Claude Code glob, не ручной |
| Prompt injection знаний | Подход 1 | Структурированный формат, категоризация |
| Hooks (existing) | Подход 3 | Для critical rules enforcement |

### Что Epic 6 НЕ делает (и правильно):
1. **Не использует MCP** — skip rate и runtime dependency не оправданы
2. **Не использует RAG** — масштаб <500 правил, pre-loading достаточен [R3]
3. **Не полагается на auto-memory** — активное управление знаниями необходимо
4. **Не компилирует знания** — project-specific, dynamic

### Потенциальные улучшения (Growth phase):
1. **BM25 keyword search** (подход 5) при >500 правил — чистый Go, 0 deps
2. **MCP Memory Server** с forced invocation через hooks — при multi-project
3. **Hooks generation** для целевого проекта — расширяет enforcement scope

---

## Ответы на конкретные вопросы

### Работает ли `claude --print` с hooks?
**Да, при условии `--allowedTools`.** Документация подтверждает: `-p` flag = headless mode, все CLI options работают. SessionStart fires при старте сессии. PreToolUse/PostToolUse fires при tool calls (которые доступны через `--allowedTools`). CLAUDE.md и `.claude/rules/` загружаются нормально.

### Работает ли MCP в `claude --print`?
**Да, с `--allowedTools "mcp__*"`.** MCP tools появляются как обычные tools. Но 56% skip rate без forced invocation делает MCP ненадёжным для autonomous agent.

### Можно ли RAG без зависимостей?
**Нет.** chromem-go — zero Go deps, но требует external embedding API (OpenAI/Ollama). Pre-computed embeddings возможны, но нестандартизированы и добавляют complexity. BM25 — ближайшая альтернатива без API dependency.

---

## Evidence from Project Research (R1/R2/R3) & Round 1 Reports

### Из R1 (Knowledge Extraction, 20 источников)

1. **Context rot универсален и нелинеен** [R1-S5]: 30-50% degradation между compact и full context. Все 18 frontier моделей деградируют. Это фундаментальное свойство transformer attention — никакой подход не "решит" проблему, только mitigation.

2. **~150-200 инструкций = практический предел** [R1-S14]: SFEIR research показал ~15 imperative правил = 94% compliance. 125+ правил в одном файле = ~40-50% compliance. **Для нового проекта:** начинаем с 0 правил → градиентный рост. Simple append (подход 1) работает пока правил <100, затем нужна distillation.

3. **Shuffled/atomized > organized** [R1-S5]: Перемешанные контексты outperform organized. Для LEARNINGS.md: atomized facts эффективнее structured narrative. Epic 6 format (`## category: topic [citation]`) = правильный.

4. **Convergent evolution** [R1-S9,S10,S11]: Три независимых проекта (claude-mem, Claudeception, continuous-learning) пришли к одной архитектуре: capture → compress → inject on demand. Epic 6 реализует тот же паттерн.

### Из R2 (Knowledge Enforcement, 40 источников)

5. **Тройной барьер compliance** [R2-4.1]: (1) compaction уничтожает правила, (2) context rot снижает внимание на 30-50%, (3) >15 правил = ниже порога compliance. **Для нового проекта:** hooks-based enforcement (подход 3) — единственный детерминистический механизм.

6. **56% MCP skip rate** [R2-S31]: Claude пропускает MCP tools в >50% случаев без forced invocation. **Подтверждает:** подход 2 (MCP) ненадёжен для autonomous agent.

7. **Skills activation 20% → 84% с hooks** [R2-S27]: Без forced evaluation hooks, skills используются в ~20% случаев. С hooks — ~84%. **Подтверждает:** push-модель >> pull-модель для knowledge delivery.

8. **DGM: concrete violations >> abstract rules** [R2-S37]: Хранение истории неудачных попыток = 2.5x improvement (SWE-bench 20% → 50%). **Для нового проекта:** по мере накопления violations, Epic 6 violation tracking становится мощным.

### Из R3 (Alternative Methods, 22 источника)

9. **Filesystem agent > Mem0 Graph** [R3-S1]: Letta benchmark: filesystem = 74.0% на LoCoMo, Mem0 Graph = 68.5%. File-based подход побеждает специализированные memory tools.

10. **RAG break-even при >500 entries** [R3-4.1]: Pre-loading 200 строк = ~3000-4000 tokens — within sweet spot. RAG экономит tokens но добавляет complexity и latency. **Подтверждает:** подход 6 (RAG) преждевременен.

11. **chromem-go = единственный viable pure-Go** [R3-S3]: Zero deps, CGO_ENABLED=0, но требует embedding API. **Подтверждает:** RAG без внешних зависимостей невозможен.

12. **MCP pipe mode = unverified** [R3-5.3]: На момент R3 MCP доступность в `claude --print` не подтверждена. **Обновление в этом отчёте:** MCP РАБОТАЕТ с `--allowedTools`, но 56% skip rate остаётся проблемой.

### Из analyst-1 (Round 1): Двойная инъекция

13. **Bug #16299** [analyst-1-S2]: `paths:` frontmatter в `.claude/rules/` СЛОМАН — все файлы грузятся глобально. **Критически важно для подхода 4 (lazy loading):** нативный `.claude/rules/` glob scoping НЕ РАБОТАЕТ как ожидается. Все файлы = always loaded.

14. **Двойная инъекция** [analyst-1-2.1]: Если Ralph пишет файлы в `.claude/rules/` И инжектирует через `__RALPH_KNOWLEDGE__` — контент загружается ДВАЖДЫ. ~7-12K потерянных токенов. **Для нового проекта:** подход `.ralph/rules/` (Go injection) = единственный канал, без дублирования.

15. **CVE risk** [analyst-1-A12]: CVE-2025-59536, CVE-2026-21852 — `.claude/` = confirmed attack surface. Программатическая запись Ralph-ом увеличивает risk. **Для нового проекта:** `.ralph/` изолирован от attack surface.

### Из analyst-3 (Round 1): Конкурентный анализ

16. **Glob-scoped правила = индустриальный консенсус** [analyst-3-4.1]: 4 из 5 лидеров (Cursor, Copilot, Claude Code, Continue.dev) используют YAML frontmatter + glob. Но Bug #16299 ломает это для Claude Code.

17. **Дистилляция = нерешённая проблема** [analyst-3-4.1]: Только Cline (Auto-Compact) имеет явный механизм. Devin признаёт потерю деталей. **bmad-ralph Epic 6 distillation** = ahead of industry.

18. **bmad-ralph уже best-in-class** [analyst-3-4.3]: Violation tracker + escalation, extraction protocol, research-backed подход — ни у одного конкурента нет аналогов.

### Из analyst-7 (Round 1): 5-tier система

19. **5-tier архитектура = state-of-the-art** [analyst-7-r1-5.1]: bmad-ralph: T1 (SessionStart) → T1.5 (CLAUDE.md) → T2 (rules) → T2.5 (hooks) → T3 (review). Ни один конкурент не имеет 5 уровней.

20. **BM25 keyword mismatch** [analyst-7-r1-1.1]: "Doc comment claims must match reality" vs запрос "я обновил функцию" — BM25 score ≈ 0. Lexical matching не работает для code knowledge rules с синонимами. **Подтверждает:** подход 5 (BM25) имеет серьёзные ограничения accuracy.

21. **Demotion policy отсутствует** [analyst-7-r1-5.3]: Правила только повышаются (escalation), но не понижаются. Нужен decay после N stories без нарушений. **Для нового проекта:** demotion важен для предотвращения bloat T1 по мере роста базы знаний.

### Пересмотр оценок с учётом "новый проект с нуля"

| # | Подход | Оценка (до) | Оценка (после evidence) | Изменение |
|---|--------|-------------|--------------------------|-----------|
| 1 | Simple append | 6/10 | 6/10 | = (R1 context rot подтверждает ceiling) |
| 2 | MCP tool | 4/10 | 3/10 | -1 (R2: 56% skip + R3: pipe mode risk) |
| 3 | Hooks-based | 8/10 | 8/10 | = (R2 triple barrier + Bug #16299 усиливает ценность hooks) |
| 4 | Lazy loading | 5/10 | 4/10 | -1 (Bug #16299: glob scoping broken) |
| 5 | BM25/keyword | 6/10 | 5/10 | -1 (analyst-7-r1: keyword mismatch для code rules) |
| 6 | Embeddings/RAG | 3/10 | 3/10 | = (R3 подтверждает: не нужен при <500) |
| 7 | Compilation | 2/10 | 2/10 | = (фундаментально неприменим) |
| 8 | Ничего не делать | 3/10 | 2/10 | -1 (R2: passive = insufficient, analyst-3: only 3/12 tools have persistent memory) |

### Ключевой вывод из перекрёстной верификации

**Epic 6 гибридный подход (extract → categorize → distill → inject) подтверждён ВСЕМИ источниками:**
- R1: convergent evolution к capture → compress → inject
- R2: hooks enforcement = единственный детерминистический path
- R3: file-based = optimal при <500 rules
- analyst-1: .ralph/rules/ > .claude/rules/ (избежание двойной инъекции)
- analyst-3: bmad-ralph already best-in-class среди конкурентов
- analyst-7 (r1): 5-tier > все исследованные альтернативы

**Для нового проекта с нуля:** Epic 6 design работает from day 0. LEARNINGS.md создаётся пустым. Правила накапливаются через code reviews. Distillation запускается при пороге 150 строк. `.ralph/rules/ralph-{category}.md` файлы создаются при distillation. Hooks enforce critical rules. Никакая предварительная инфраструктура не требуется.

---

## Приложение: Источники

Основные источники из трёх предшествующих исследований (R1-R3):

| ID | Ключевой вклад |
|----|---------------|
| R1-S4 | Claude Code 6-level memory hierarchy, 200-line limit |
| R1-S5 | Context rot: 30-50% degradation, 18 models |
| R1-S13 | Claude Code hooks: 12 lifecycle events, SessionStart fires on compact |
| R2-S25 | ~15 rules = 94% compliance (SFEIR) |
| R2-S31 | MCP tool 56% skip rate |
| R2-S37 | DGM: concrete violations >> abstract rules (2.5x) |
| R3-S1 | Letta: filesystem agent 74.0% > Mem0 Graph 68.5% |
| R3-S3 | chromem-go: zero deps, CGO_ENABLED=0, 0.5ms/1K docs |
| R3-S4 | MCP Memory Server: knowledge graph, JSONL, 9 CRUD tools |

Web-исследование:
- [Claude Code headless mode docs](https://code.claude.com/docs/en/headless) — `-p` = headless, all CLI options work
- [Claude Code hooks reference](https://code.claude.com/docs/en/hooks) — hooks fire on tool events
- [Claude Code settings](https://code.claude.com/docs/en/settings) — allowedTools configuration
