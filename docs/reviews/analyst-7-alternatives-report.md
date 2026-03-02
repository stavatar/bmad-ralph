# Нестандартные подходы к управлению знаниями CLI-агента

**Аналитик:** analyst-7
**Дата:** 2026-03-02
**Контекст:** bmad-ralph — Go CLI, single-process, 3 зависимости (cobra, yaml.v3, fatih/color), Claude Code runtime
**База знаний:** ~122 правил в `.claude/rules/` (7 topic-файлов), ~65 строк CLAUDE.md, 15 critical rules в SessionStart hook

---

## Executive Summary

Исследованы 7 подходов к управлению знаниями для CLI-агента bmad-ralph. **Файловая инъекция с glob-scoped правилами остаётся оптимальной** для текущего масштаба (~122 правил). Из нестандартных подходов наибольший потенциал имеют:

1. **Hierarchical prompting (Tier 1/2/3)** — уже частично реализован в bmad-ralph, требует формализации (оценка: 9.1/10)
2. **Tool-based knowledge via hooks** — PreToolUse уже работает, расширение до полноценной event-driven системы (оценка: 8.5/10)
3. **MCP tool "get_knowledge"** — перспективен для Growth phase (>300 правил), но зависит от MCP в pipe mode (оценка: 7.3/10)
4. **Lazy loading / agent-initiated** — Claude Code glob-scoped rules уже реализуют pull-based паттерн (оценка: 7.8/10)
5. **Embedded BM25** — реалистичен на чистом Go (~200 LoC), но не оправдан при <500 записей (оценка: 6.2/10)
6. **Semantic routing via embeddings** — требует внешний embedding API, избыточен для текущего масштаба (оценка: 4.8/10)
7. **Инновационные подходы** (knowledge compilation, violation-frequency prioritization, freshness decay) — отдельные элементы уже встроены в violation-tracker (оценка: 7.0/10)

**Ключевой вывод:** bmad-ralph's 5-tier enforcement architecture (T1 SessionStart → T1.5 CLAUDE.md → T2 rules → T2.5 hooks → T3 review) уже представляет собой state-of-the-art для CLI-агента с <500 правилами. Дополнительная сложность RAG/embeddings/MCP не оправдана до порога ~500 правил.

---

## 1. Embedded RAG без внешних зависимостей

### 1.1. BM25 на чистом Go

**Подход:** Lexical retrieval (BM25/Okapi) для ранжирования правил по релевантности к текущей задаче. Чисто алгоритмический, без embeddings.

**Существующие Go-библиотеки:**

| Библиотека | Зависимости | Особенности |
|-----------|------------|-------------|
| [crawlab-team/bm25](https://github.com/crawlab-team/bm25) | Минимальные | Порт rank_bm25, параллельные batched-вычисления |
| [go-nlp/bm25](https://github.com/go-nlp/bm25) | Минимальные | Базовая scoring function |
| [covrom/bm25s](https://pkg.go.dev/github.com/covrom/bm25s) | Минимальные | Оптимизирован для коротких текстов, stemming |
| [iwilltry42/bm25-go](https://pkg.go.dev/github.com/iwilltry42/bm25-go/bm25) | Минимальные | 5 вариантов BM25 (Okapi, Plus, L, Adpt, T) |

**Реализация на stdlib (~150 LoC):**

```go
// bm25.go — минимальный BM25 на stdlib
package knowledge

import (
    "math"
    "strings"
)

type BM25Index struct {
    docs     [][]string          // tokenized documents
    df       map[string]int      // document frequency
    avgDL    float64             // average document length
    k1, b   float64             // BM25 parameters
}

func NewBM25Index(documents []string) *BM25Index {
    idx := &BM25Index{
        df: make(map[string]int),
        k1: 1.5, b: 0.75,
    }
    totalLen := 0
    for _, doc := range documents {
        tokens := tokenize(doc)
        idx.docs = append(idx.docs, tokens)
        totalLen += len(tokens)
        seen := make(map[string]bool)
        for _, t := range tokens {
            if !seen[t] {
                idx.df[t]++
                seen[t] = true
            }
        }
    }
    idx.avgDL = float64(totalLen) / float64(len(documents))
    return idx
}

func (idx *BM25Index) Search(query string, topK int) []int {
    qTokens := tokenize(query)
    n := float64(len(idx.docs))
    scores := make([]float64, len(idx.docs))

    for _, qt := range qTokens {
        idf := math.Log((n - float64(idx.df[qt]) + 0.5) / (float64(idx.df[qt]) + 0.5) + 1)
        for i, doc := range idx.docs {
            tf := countTerm(doc, qt)
            dl := float64(len(doc))
            score := idf * (tf * (idx.k1 + 1)) / (tf + idx.k1*(1-idx.b+idx.b*dl/idx.avgDL))
            scores[i] += score
        }
    }
    return topKIndices(scores, topK)
}

func tokenize(s string) []string {
    return strings.Fields(strings.ToLower(s))
}
```

**Оценка для bmad-ralph:**

| Критерий | Оценка | Комментарий |
|----------|--------|-------------|
| Сложность реализации | Низкая (~150-200 LoC) | Stdlib-only, без зависимостей |
| Эффективность context window | Средняя | Загружает top-K вместо всех |
| Accuracy/recall | **Низкая для code rules** | BM25 = lexical match, "doc comments" не найдёт "stale documentation" |
| Latency | <1ms на 500 документов | Brute-force BM25 очень быстр |
| Зависимости | 0 | Чистый Go stdlib |
| Fit для проекта | **Слабый** | Code rules используют разную терминологию для одних концепций |

**Проблема BM25 для code knowledge:** Правила bmad-ralph используют синонимы и переформулировки. Пример:
- Правило: "Doc comment claims must match reality"
- Запрос агента: "я обновил функцию и поменял её поведение"
- BM25 score ≈ 0 (нет общих терминов)

BM25 работает для exact keyword match, но code knowledge rules требуют **семантического** понимания.

### 1.2. TF-IDF на stdlib

Аналогичен BM25, но проще. TF-IDF не учитывает длину документа и term saturation. Для коротких правил (2-4 строки) разница с BM25 минимальна. Те же проблемы с синонимами.

### 1.3. Cosine similarity с pre-computed embeddings

**Подход:** Embeddings генерируются однократно (при добавлении правила), хранятся в JSON-файле. При запросе — cosine similarity на чистом Go.

```go
// cosine.go — stdlib cosine similarity
func cosineSimilarity(a, b []float64) float64 {
    var dot, normA, normB float64
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**Проблема:** Embeddings нужно откуда-то взять. Варианты:
- OpenAI API ($0.02/1M tokens) — внешняя зависимость
- Ollama (local) — требует установки
- Встроенная модель — невозможно в Go без CGO

**Вердикт:** Cosine similarity на Go = тривиально. Генерация embeddings = блокер. Нет pure-Go embedding модели, совместимой с CGO_ENABLED=0.

### 1.4. Общий вердикт по embedded RAG

**При <500 правилах RAG не оправдан.** Pre-loading ~122 правил = ~3000-4000 tokens. Это within sweet spot для context window. RAG экономит ~2000 tokens, но добавляет:
- Риск пропуска релевантного правила (recall < 100%)
- Зависимость (embedding API для semantic, синонимы для BM25)
- ~200-300 LoC дополнительного кода

Подтверждено бенчмарком Letta: filesystem agent = 74.0% на LoCoMo, Mem0 Graph = 68.5%. Файловая система побеждает специализированные memory tools.

---

## 2. Lazy Loading / Agent-Initiated Knowledge Retrieval

### 2.1. Push vs Pull модели

| Модель | Механизм | Пример в bmad-ralph | Compliance |
|--------|----------|---------------------|------------|
| **Push (eager)** | Правила загружаются системой при старте | SessionStart hook → critical-rules.md | ~90-94% |
| **Push (conditional)** | Правила загружаются по glob-match | `.claude/rules/test-*.md` при редактировании `*_test.go` | ~60-90% |
| **Pull (explicit)** | Агент запрашивает категорию | `Read .claude/rules/test-error-patterns.md` | ~30-50% |
| **Pull (implicit)** | Агент ищет по ключевому слову | `Grep "error wrapping" .claude/rules/` | ~20-40% |

### 2.2. Claude Code glob-scoped rules — уже частичный lazy loading

Claude Code `.claude/rules/` система **уже реализует conditional push**:
- Файлы в `.claude/rules/` загружаются с framing "may or may not be relevant"
- Scope comments (`# Scope: core assertion patterns for any Go test`) — подсказка Claude
- При редактировании `*_test.go` — Claude видит test-related rules с приоритетом

Это **не чистый pull** (Claude не запрашивает), но и **не чистый push** (не всё загружается всегда). Это **conditional push** — система решает что загрузить на основе контекста файла.

### 2.3. Чистый pull-based подход

**Идея:** Агент получает только index (TOC) правил. Когда нужно — явно запрашивает категорию.

```
# .claude/rules/knowledge-index.md (всегда загружен, ~20 строк)
Available knowledge categories:
- test-naming: naming conventions for Go tests (12 rules)
- test-errors: error testing patterns (11 rules)
- test-assertions: count, substring, symmetric checks (23 rules)
- code-quality: doc comments, DRY, sentinels (28 rules)
- wsl-ntfs: WSL/NTFS file system patterns (12 rules)

To load a category: Read .claude/rules/<category>.md
```

**Проблемы:**
1. **Claude не знает что ему нужно** до начала работы. Правило "doc comments must match reality" нужно ДО редактирования, не после
2. **Compliance drop**: SFEIR research показал ~40-50% compliance для "agent decides" vs ~90-94% для "system injects"
3. **Latency**: каждый `Read` = дополнительный tool call (~200ms)

### 2.4. Гибрид: eager critical + lazy detailed

**Оптимальная модель (уже реализована в bmad-ralph):**
- T1 (always push): 15 critical rules via SessionStart — не зависит от агента
- T2 (conditional push): glob-scoped rules — система решает по контексту файла
- T2.5 (event push): PreToolUse checklist — триггерится перед Edit/Write
- T3 (pull): агент может прочитать любой rules-файл если нужно

**Вердикт:** Чистый lazy loading **снижает compliance**. bmad-ralph уже реализует оптимальный гибрид push+pull. Дальнейший сдвиг к pull нежелателен — правила нужны ДО ошибки, не после.

---

## 3. MCP Tool "get_knowledge(category)"

### 3.1. Архитектура

```
┌──────────────────┐     MCP stdio      ┌─────────────────────┐
│   Claude Code    │◄──────────────────►│  Knowledge MCP      │
│   (host)         │   tool calls        │  Server (Go)        │
│                  │                     │                     │
│  get_knowledge() │───────────────────►│  BM25 / category    │
│  search_rules()  │◄───────────────────│  index + files      │
│  report_violation│───────────────────►│  violation tracker   │
└──────────────────┘                     └─────────────────────┘
```

### 3.2. Go MCP Server реализация

Официальный Go SDK: [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) (maintained совместно с Google).
Альтернатива: [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go).

```go
// mcp_knowledge_server.go — скетч
package main

import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
    server := mcp.NewServer("ralph-knowledge", "1.0.0")

    server.AddTool("get_knowledge", mcp.ToolDef{
        Description: "Retrieve knowledge rules by category",
        InputSchema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "category": map[string]any{
                    "type": "string",
                    "enum": []string{"test-naming", "test-errors",
                        "test-assertions", "code-quality", "wsl-ntfs"},
                },
            },
        },
    }, handleGetKnowledge)

    server.AddTool("search_rules", mcp.ToolDef{
        Description: "Search rules by keyword",
        InputSchema: map[string]any{...},
    }, handleSearchRules)

    // stdio transport
    server.ServeStdio()
}

func handleGetKknowledge(args map[string]any) (string, error) {
    category := args["category"].(string)
    content, err := os.ReadFile(".claude/rules/" + category + ".md")
    return string(content), err
}
```

### 3.3. Трейдоффы

| Аспект | Значение |
|--------|---------|
| **Context window** | Отлично — загружает только запрошенное |
| **Accuracy** | Зависит от агента — может не запросить нужную категорию |
| **Latency** | ~50-200ms per tool call (MCP stdio overhead) |
| **Зависимости** | +1 Go dependency (MCP SDK) + отдельный процесс |
| **Complexity** | ~300-500 LoC для сервера |
| **Критический блокер** | **MCP доступность в `claude --print` mode не подтверждена** |

### 3.4. Преимущества над файловой инъекцией

1. **Динамический retrieval**: агент запрашивает по контексту, не по glob-паттерну
2. **Structured tools**: `search_rules(query="error wrapping")` > grep
3. **Bidirectional**: агент может **записывать** violations через `report_violation` tool
4. **Analytics**: сервер логирует какие категории запрашиваются чаще

### 3.5. Критические проблемы

1. **MCP в pipe mode**: `claude --print` / `claude --resume` может не загружать MCP серверы — **не верифицировано**
2. **Compliance drop**: Pull-based retrieval = агент решает что запрашивать = ~30-50% compliance vs push 90-94%
3. **Дополнительный процесс**: MCP сервер = отдельный binary, lifecycle management
4. **Dependency**: +1 зависимость (MCP Go SDK) нарушает "only 3 deps" правило

### 3.6. Вердикт

MCP knowledge tool — **перспективен для Growth phase (>300 правил)**, когда:
- Верифицирована MCP доступность в pipe mode
- Файловая инъекция начинает saturate context window
- Можно комбинировать: critical rules (push) + detailed rules (MCP pull)

**Сейчас:** преждевременная оптимизация. 122 правила = 3-4K tokens = не проблема.

---

## 4. Semantic Routing via Embeddings

### 4.1. Концепция

Pre-computed embeddings для каждого правила. При поступлении задачи — embed задачу, найти top-K ближайших правил по cosine similarity.

```
┌─────────────┐   embed query   ┌──────────────┐   cosine    ┌──────────────┐
│ Task text    │───────────────►│ Query vector  │───────────►│ Top-K rules  │
└─────────────┘                 └──────────────┘  similarity  └──────────────┘
                                                      ▲
                                      ┌───────────────┘
                                      │ Pre-computed
                               ┌──────┴──────┐
                               │ Rule vectors │  (JSON file)
                               │ 122 × 1536d │
                               └─────────────┘
```

### 4.2. Embedding generation

| Вариант | Latency | Стоимость | Offline | Зависимости |
|---------|---------|-----------|---------|-------------|
| OpenAI text-embedding-3-small | 100-300ms | $0.02/1M tokens | Нет | API key |
| Ollama nomic-embed-text | 50-200ms | Бесплатно | Да | Ollama install |
| Встроенная Go-модель | N/A | N/A | N/A | **Не существует для CGO_ENABLED=0** |

**Блокер:** Нет чисто Go embedding модели без CGO. Любой вариант требует внешний сервис.

### 4.3. Storage format

Для 122 правил × 1536 dimensions (OpenAI small):
- JSON: ~1.5 MB (readable, portable)
- Binary (gob): ~750 KB (faster load)
- В памяти: ~750 KB (trivial)

Cosine similarity brute-force на 122 векторах × 1536d: **<0.1ms** на любом CPU.

### 4.4. Когда semantic routing оправдан

| Условие | Оценка для bmad-ralph |
|---------|----------------------|
| >500 правил | Нет (122 сейчас) |
| Правила с синонимами | Да, но не критично при push-модели |
| Heterogeneous knowledge | Частично (7 категорий) |
| Высокий % unused rules per session | Нет (большинство правил потенциально релевантно) |

### 4.5. Вердикт

**Не оправдан при текущем масштабе.** Semantic routing решает проблему "найти нужное в большом корпусе". При 122 правилах и push-модели (система загружает по glob) — проблемы нет. Embedding API = внешняя зависимость, нарушающая constraint "minimal dependencies".

**Может стать релевантным** при >1000 правил, когда даже distilled knowledge не помещается в context window целиком.

**Оценка: 4.8/10** — технически реализуемо, но добавляет complexity без benefit при текущем масштабе.

---

## 5. Hierarchical Prompting / Tiered Knowledge

### 5.1. Текущая 5-tier архитектура bmad-ralph

bmad-ralph **уже реализует** наиболее продвинутую из исследованных моделей:

```
Tier 1 (Critical)     SessionStart hook → critical-rules.md
                      15 правил + violation examples
                      Выживает compaction, без framing "may or may not be relevant"
                      Compliance: ~90-94%

Tier 1.5 (Core)      CLAUDE.md (~65 строк)
                      ~25 правил, eager-loaded
                      Framing: "may or may not be relevant"
                      Частично выживает compaction
                      Compliance: ~70-80%

Tier 2 (Topic)       .claude/rules/*.md (9 файлов, ~122 правила)
                      Glob-scoped, ~12-23 правил на файл
                      НЕ выживает compaction
                      Compliance: ~60-90% (зависит от размера файла)

Tier 2.5 (Active)    PreToolUse checklist (Go файлы)
                      PostToolUse CRLF fix (все файлы)
                      PreToolUse: ~95% compliance через explicit checklist
                      PostToolUse CRLF: 100% deterministic

Tier 3 (Review)      Code review workflow + verify-knowledge-extraction.sh
                      Знания обнаруженные post-hoc
                      Feed back в Tiers 1-2.5 через escalation
```

### 5.2. Сравнение с industry

| Система | Tiers | Self-editing | Compaction survival | Escalation |
|---------|-------|-------------|---------------------|-----------|
| **bmad-ralph** | **5** | Нет (Go validates) | T1, частично T1.5 | **Да (violation-tracker)** |
| MemGPT/Letta | 3 | Да (agent edits core) | N/A (hosted) | Нет |
| MemOS | 3 | Да | N/A (hosted) | Нет |
| Claude Code vanilla | 2 | Нет | CLAUDE.md only | Нет |
| GitHub Copilot | 1 | Нет | N/A | Нет |

**Ключевой differentiator bmad-ralph:** violation-based escalation. Правило, нарушаемое 6+ раз, автоматически повышается до T1 (SessionStart hook). Ни одна из исследованных систем не имеет аналогичного механизма.

### 5.3. Потенциальные улучшения

**5.3.1. Формализация tier promotion API:**

```go
// knowledge/escalation.go
type EscalationPolicy struct {
    ThresholdT2toT1_5 int  // default: 3 occurrences
    ThresholdT1_5toT1 int  // default: 6 occurrences
    CooldownPeriod    int  // stories between re-evaluation
}

func (p *EscalationPolicy) ShouldPromote(rule Rule) Tier {
    switch {
    case rule.ViolationCount >= p.ThresholdT1_5toT1:
        return TierCritical
    case rule.ViolationCount >= p.ThresholdT2toT1_5:
        return TierCore
    default:
        return TierTopic
    }
}
```

**5.3.2. Demotion policy (отсутствует):**
Правила только повышаются. Нет механизма понижения правила, которое перестало нарушаться. Добавление demotion (после N stories без нарушений → понизить на 1 tier) предотвратит bloat T1.

**5.3.3. Dynamic tier boundary:**
Текущие пороги (3/6) — фиксированные. Адаптивные пороги на основе общего количества правил (% от total) были бы robust.

### 5.4. Вердикт

**bmad-ralph's hierarchical system — best-in-class для CLI-агента.** Уникальные преимущества:
- 5 tier-ов (vs 2-3 у конкурентов)
- Violation-based escalation (unique)
- Compaction-resistant T1 via SessionStart hook (validated by R2 research)
- Active enforcement via PreToolUse (95% compliance)

**Рекомендация:** Формализовать demotion policy. Всё остальное — keep as-is.

**Оценка: 9.1/10** — уже реализован, минимальные улучшения нужны.

---

## 6. Tool-Based Knowledge Injection

### 6.1. Текущие hook-based механизмы

bmad-ralph уже использует 3 hook-точки:

| Hook | Событие | Механизм | Что инъектирует |
|------|---------|----------|----------------|
| SessionStart | startup, resume, /clear, compact | `cat critical-rules.md` | 15 critical rules |
| PreToolUse (Edit\|Write) | перед редактированием `.go` | `pre-edit-checklist.sh` | 6-item checklist |
| PostToolUse (Edit\|Write) | после редактирования | `fix-crlf.sh` | CRLF fix (deterministic) |

### 6.2. Расширение: event-driven knowledge

**Идея:** Расширить hook-based injection до полноценной event-driven системы знаний.

**Новые trigger-точки (теоретические):**

| Событие | Knowledge injection | Benefit |
|---------|-------------------|---------|
| PreToolUse `Read` на `*_test.go` | Testing patterns summary | Правила перед чтением тестов |
| PreToolUse `Bash` с `go test` | Common test failures + fixes | Помощь при failing tests |
| PostToolUse `Bash` с exit code ≠ 0 | Debugging patterns для ошибки | Context-aware troubleshooting |
| PreToolUse `Write` на `*.md` | Documentation standards | Правила для markdown |
| PostToolUse `Edit` на файл с `func Test` | "Did you update golden files?" | Reminder после изменения тестов |

### 6.3. Преимущества tool-based подхода

1. **Точечная доставка**: знания приходят в момент, когда они нужны (JIT)
2. **Не pollute context window**: additionalContext ≠ system prompt
3. **Deterministic triggers**: событие → знание (не зависит от воли агента)
4. **Composable**: каждый hook = независимый модуль
5. **Measurable**: можно отслеживать какие hooks сработали и помогли

### 6.4. Ограничения

1. **Claude Code hook API**: только `command` type в hooks (bash scripts). Нельзя сделать "smart" routing из bash
2. **Overhead**: каждый hook = fork + exec bash script (~10-50ms)
3. **Granularity**: matcher паттерны ограничены (tool name regex, не file content)
4. **additionalContext framing**: Claude Code может интерпретировать по-разному

### 6.5. Архитектурный скетч расширенной системы

```bash
#!/bin/bash
# .claude/hooks/knowledge-router.sh
# Unified knowledge router — вызывается из PreToolUse для всех инструментов

INPUT=$(cat)
TOOL=$(echo "$INPUT" | jq -r '.tool_name')
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

case "$TOOL:${FILE##*.}:$CMD" in
  Edit:go:*|Write:go:*)
    cat .claude/hooks/knowledge/go-edit-checklist.md
    ;;
  Read:go:*)
    # Если читает тест — подгрузить testing patterns summary
    if [[ "$FILE" == *_test.go ]]; then
      cat .claude/hooks/knowledge/test-patterns-summary.md
    fi
    ;;
  Bash:*:*go\ test*)
    cat .claude/hooks/knowledge/test-debugging.md
    ;;
esac
```

### 6.6. Вердикт

Tool-based injection через hooks — **наиболее практичное расширение** текущей архитектуры:
- Не требует новых зависимостей
- Использует существующую infrastructure (hooks)
- JIT-доставка = высокая compliance без context pollution
- Единственное ограничение — granularity matcher-ов в Claude Code hooks API

**Оценка: 8.5/10** — расширяет proven mechanism, low risk, high benefit.

---

## 7. Инновационные подходы

### 7.1. Knowledge Compilation

**Идея:** На этапе `make build` или `ralph distill` — скомпилировать все правила в оптимизированный формат:

```go
// generated_knowledge.go (auto-generated)
package knowledge

var CompiledRules = map[string][]Rule{
    "test-naming": {
        {ID: "TN-1", Text: "Test<Type>_<Method>_<Scenario>", Priority: 9, ViolationCount: 32},
        {ID: "TN-2", Text: "Zero-value tests: one function per type", Priority: 7, ViolationCount: 5},
    },
    // ...
}
```

**Преимущества:**
- Zero runtime parsing
- Validation at build time
- Type-safe access
- Can include pre-computed metadata (violation counts, priorities)

**Ограничения:**
- Requires rebuild after knowledge change
- Не работает для dynamic knowledge (LEARNINGS.md)
- Обходной путь: `go generate` + file watcher

### 7.2. Violation-Frequency Prioritization

**Идея:** Правила, нарушаемые чаще, получают приоритет в injection. Уже частично реализовано через violation-tracker escalation.

**Расширение — weighted injection:**

```go
type WeightedRule struct {
    Text           string
    ViolationRate  float64  // violations per story
    LastViolated   time.Time
    DecayFactor    float64  // снижение приоритета со временем
}

func (r *WeightedRule) Priority() float64 {
    daysSince := time.Since(r.LastViolated).Hours() / 24
    return r.ViolationRate * math.Exp(-r.DecayFactor * daysSince)
}
```

**Применение:** При ограниченном context budget — загружать правила в порядке убывания Priority(). Высокочастотные + недавние правила загружаются первыми.

**bmad-ralph уже имеет:** violation-tracker.md с частотами. Escalation thresholds (3/6) = грубая версия этого подхода. Полная формализация добавит ~100 LoC.

### 7.3. Knowledge Freshness / Decay

**Идея:** Правила "стареют" если долго не нарушаются. Старые правила понижаются в tier или архивируются.

**Модель decay:**
- `freshness = max_priority * exp(-lambda * days_since_last_violation)`
- lambda = 0.01 (медленный decay, ~70 дней до half-life)
- Правило с 0 нарушениями за 3 epic-а → кандидат на demotion

**Риск:** Правило может быть critical но не нарушаться ПОТОМУ ЧТО оно в T1. Понижение → нарушение. Нужен cooldown period и monitoring.

### 7.4. Self-Pruning Knowledge Base

**Идея:** Автоматическое удаление правил, которые:
1. Никогда не нарушались (потенциально очевидные)
2. Дублируют другие правила (semantic dedup)
3. Устарели (код, к которому относятся, удалён)

**Реализация для bmad-ralph:**
```bash
# Найти правила без [file:line] citation — потенциально устаревшие
grep -L '\[.*\.go\]' .claude/rules/test-*.md

# Найти citation references к удалённым файлам
for f in $(grep -oP '\[\w+/[\w.]+\]' .claude/rules/*.md | sort -u); do
    file=$(echo "$f" | tr -d '[]')
    [ ! -f "$file" ] && echo "STALE: $f"
done
```

### 7.5. Context-Aware Rule Grouping

**Идея:** Вместо загрузки по file glob, группировать правила по "ситуации":

| Ситуация | Rule group | Trigger |
|----------|-----------|---------|
| "Writing new test" | test-naming + test-structure + test-mocks | Creating `*_test.go` |
| "Fixing test failure" | test-error-patterns + test-assertions + debugging | `go test` exit != 0 |
| "Refactoring function" | code-quality + doc-comments + error-wrapping | Editing function signature |
| "Adding new feature" | architecture + naming + YAGNI | Creating new `.go` file |

Это расширение hook-based подхода (раздел 6) с семантическими группами.

### 7.6. Вердикт по инновационным подходам

| Подход | Effort | Benefit | Рекомендация |
|--------|--------|---------|-------------|
| Knowledge compilation | Средний | Средний | Рассмотреть для stable rules |
| Violation-frequency priority | Низкий | Высокий | **Реализовать** (расширение violation-tracker) |
| Freshness decay | Низкий | Средний | Реализовать demotion policy |
| Self-pruning | Средний | Средний | Полуавтоматический (script + human review) |
| Context-aware grouping | Средний | Высокий | **Реализовать** через hooks |

**Оценка: 7.0/10** — отдельные элементы уже встроены, формализация добавит ценность.

---

## 8. Сводная матрица оценок

| Подход | Complexity | Context Window | Accuracy | Latency | Deps | Project Fit | **Score** |
|--------|-----------|---------------|----------|---------|------|-------------|-----------|
| **5. Hierarchical (current)** | 2/10 | 8/10 | 9/10 | 10/10 | 0 | 10/10 | **9.1** |
| **6. Tool-based hooks** | 3/10 | 9/10 | 8/10 | 9/10 | 0 | 9/10 | **8.5** |
| **4. Lazy loading (hybrid)** | 3/10 | 9/10 | 6/10 | 8/10 | 0 | 8/10 | **7.8** |
| **3. MCP knowledge tool** | 5/10 | 9/10 | 7/10 | 7/10 | +1 | 6/10 | **7.3** |
| **7. Innovative (composite)** | 4/10 | 7/10 | 7/10 | 9/10 | 0 | 8/10 | **7.0** |
| **1. BM25 (pure Go)** | 4/10 | 8/10 | 5/10 | 10/10 | 0 | 5/10 | **6.2** |
| **1. Semantic (embeddings)** | 6/10 | 9/10 | 8/10 | 7/10 | +1 API | 3/10 | **4.8** |

**Веса:** Project Fit (25%), Accuracy (20%), Context Window (15%), Complexity (15%), Latency (15%), Dependencies (10%)

---

## 9. Рекомендации

### R1: Сохранить 5-tier hierarchical system (CRITICAL)

Текущая архитектура — best-in-class для CLI-агента с <500 правилами. Ни один из исследованных подходов не даёт material improvement при текущем масштабе.

### R2: Расширить tool-based injection через hooks (HIGH)

Добавить context-aware knowledge groups в PreToolUse hooks:
- `*_test.go` → testing patterns summary
- `go test` fail → debugging patterns
- New `.go` file → architecture + naming rules

### R3: Формализовать demotion policy в violation-tracker (MEDIUM)

Добавить decay/demotion для правил, не нарушавшихся 3+ epic-а. Предотвращает bloat T1.

### R4: MCP knowledge tool — defer до Growth phase (LOW)

Верифицировать MCP в `claude --print` mode. Если работает — план миграции Tier 3 на MCP. Если нет — file-based остаётся единственным путём.

### R5: RAG (BM25/embeddings) — defer до >500 правил (INFORMATIONAL)

Ни BM25 (lexical mismatch), ни embeddings (внешняя зависимость) не оправданы при <500 записях. Пересмотреть при доказанной неэффективности file-based подхода.

---

## Источники

| ID | Описание | URL |
|----|----------|-----|
| W1 | Claude Code MCP Server Setup Guide | https://www.ksred.com/claude-code-as-an-mcp-server-an-interesting-capability-worth-understanding/ |
| W2 | Claude Code Hooks Reference | https://code.claude.com/docs/en/hooks |
| W3 | Official Go SDK for MCP | https://github.com/modelcontextprotocol/go-sdk |
| W4 | mcp-go Community SDK | https://github.com/mark3labs/mcp-go |
| W5 | MCP Server in Go tutorial | https://prasanthmj.github.io/ai/mcp-go/ |
| W6 | BM25 for RAG guide | https://www.ai-bites.net/tf-idf-and-bm25-for-rag-a-complete-guide/ |
| W7 | crawlab-team/bm25 Go library | https://github.com/crawlab-team/bm25 |
| W8 | covrom/bm25s optimized for short texts | https://pkg.go.dev/github.com/covrom/bm25s |
| W9 | chromem-go vector database | https://github.com/philippgille/chromem-go |
| W10 | Progressive Disclosure in Agentic Workflows | https://medium.com/@prakashkop054/s01-mcp03-progressive-disclosure-for-knowledge-discovery-in-agentic-workflows-8fc0b2840d01 |
| W11 | Progressive Disclosure is the Soul of Skills | https://dev.to/miaoshuyo/progressive-disclosure-is-the-soul-of-skills-5bi1 |
| W12 | Agents and Large Files — Progressive Disclosure | https://lethain.com/agents-large-files/ |
| W13 | Agent Skills — Procedural Memory Survey | https://www.techrxiv.org/users/1016212/articles/1376445 |
| W14 | Memory Management for Long-Running Agents | https://arxiv.org/pdf/2509.25250 |
| W15 | Claude Code Hooks Mastery | https://github.com/disler/claude-code-hooks-mastery |
| R3 | bmad-ralph R3: Alternative Knowledge Methods | `docs/research/alternative-knowledge-methods-for-cli-agents.md` |
| R2 | bmad-ralph R2: Knowledge Enforcement | `docs/research/knowledge-enforcement-in-claude-code-agents.md` |
| R1 | bmad-ralph R1: Knowledge Extraction | `docs/research/knowledge-extraction-in-claude-code-agents.md` |
