# Пара 3 — Архитектурный отчёт: Injection Pipeline, Serena Integration, Integration Testing

**Дата:** 2026-03-02
**Роль:** Архитектор, Пара 3 (adversarial review Epic 6 v5)
**Scope:** Stories 6.2, 6.7, 6.8 + cross-cutting: 2-stage template, MCP detection, prompt size, integration strategy
**Метод:** Deep research (MCP integration patterns, prompt architectures, context budgeting 2024-2026) + code analysis + decision log review

---

## 1. Архитектурные проблемы

### [A3-CRITICAL-1] Stage 2 injection не имеет бюджетного контроля — prompt overflow неизбежен

**Severity: CRITICAL**

`config/prompt.go:61-87` — `AssemblePrompt` делает `strings.ReplaceAll` для каждого ключа в `replacements` map. Ни один этап не проверяет суммарный размер результата.

Story 6.2 добавляет 2 новых placeholder'а: `__LEARNINGS_CONTENT__` и `__RALPH_KNOWLEDGE__`. Вместе с существующими `__TASK_CONTENT__`, `__FORMAT_CONTRACT__`, `__STORY_CONTENT__`, `__EXISTING_TASKS__`, `__FINDINGS_CONTENT__` — это 7+ injected блоков.

**Проблема:** LEARNINGS.md может расти до 300+ строк (по дизайну: "300+ строк = 3-4% контекста"). ralph-*.md файлов может быть 7+ (по canonical categories). При 7 файлах по 30 строк + 300 строк raw = ~500 строк знаний. Плюс task content, story content, findings — итого промпт может достичь 20-30K tokens.

**Research context:** JetBrains (NeurIPS 2025) показал, что observation masking (простое отсечение) работает не хуже LLM-summarization. SFEIR: 15 правил = 94% compliance; каждое дополнительное правило снижает compliance нелинейно. HumanLayer ACE/FIC: target 40-60% context window utilization.

**Что bmad-ralph v5 делает:** stderr warning (M6) — informational only. Не блокирует, не обрезает.

**Рекомендация:** Добавить token budget check в `AssemblePrompt` ПОСЛЕ Stage 2. Если суммарный результат > configurable threshold (например, 8000 tokens ~= 32K chars), применить graceful degradation:
1. Truncate `__LEARNINGS_CONTENT__` до last N entries
2. Если всё ещё over budget — exclude ralph-misc.md
3. Log warning с actual/budget размерами

```
func AssemblePrompt(...) (string, error) {
    // Stage 1 + Stage 2 as-is
    result := ...

    // Budget check (defense-in-depth)
    if maxChars > 0 && len(result) > maxChars {
        log.Printf("⚠ Prompt exceeds budget: %d/%d chars", len(result), maxChars)
        // Graceful degradation: caller responsibility
    }
    return result, nil
}
```

Альтернатива: budget check в caller (runner), не в AssemblePrompt. Лучше для SRP.

---

### [A3-CRITICAL-2] MCP config parsing — хрупкая схема, нет версионирования

**Severity: CRITICAL**

Story 6.7 предполагает парсинг `.claude/settings.json` и `.mcp.json` для обнаружения Serena. Research показывает:

1. **Формат НЕ стандартизирован.** GitHub Discussion #681 на modelcontextprotocol repo — активная дискуссия о формализации. Формат `mcpServers` де-факто, но каждый инструмент имеет вариации.

2. **Claude Code использует 3 scope уровня:** user (`~/.claude/`), project (`.claude/`), workspace. Story 6.7 проверяет только project-level. User-level Serena config не обнаружится.

3. **Settings.json структура может измениться.** Claude Code обновляется часто (115+ versions по Piebald-AI tracking). JSON schema settings.json не публичная.

4. **`.mcp.json` — формат Cursor**, не Claude Code. Claude Code использует `.claude/settings.json` с вложенным `mcpServers` объектом. Смешение форматов в одном AC.

**Конкурентный контекст:**
- Claude Code: `~/.claude/settings.json` (user) + `.claude/settings.json` (project)
- Cursor: `~/.cursor/mcp.json` + `.cursor/mcp.json`
- VS Code Copilot: `.vscode/mcp.json`
- Windsurf: `~/.codeium/windsurf/mcp_config.json`

**Рекомендация:**
1. Парсить ТОЛЬКО `.claude/settings.json` (project-level) — это канонический путь для Claude Code
2. Использовать defensive parsing: `json.Unmarshal` → check `mcpServers` key → search for "serena" (case-insensitive) в именах серверов
3. Fallback: если парсинг failed — `Available() = false`, no error. Best-effort design уже в AC, но нужно документировать КАКИЕ файлы и КАКИЕ paths

```go
type MCPConfig struct {
    MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

func (d *SerenaMCPDetector) Available(projectRoot string) bool {
    paths := []string{
        filepath.Join(projectRoot, ".claude", "settings.json"),
        // NOT .mcp.json — that's Cursor format
    }
    for _, p := range paths {
        data, err := os.ReadFile(p)
        if err != nil { continue }
        var cfg MCPConfig
        if json.Unmarshal(data, &cfg) != nil { continue }
        for name := range cfg.MCPServers {
            if strings.Contains(strings.ToLower(name), "serena") {
                return true
            }
        }
    }
    return false
}
```

---

### [A3-HIGH-1] Integration test strategy — 11 scenarios в одном файле, нет isolation

**Severity: HIGH**

Story 6.8 определяет 11 `Scenario: FINAL —` в одном `runner_final_integration_test.go`. Каждый scenario покрывает полный E2E flow. Проблемы:

1. **Файл будет 1500-2000 строк.** 11 scenarios × ~150 строк каждый (setup + mocks + assertions). По pattern из existing integration tests (`runner_integration_test.go`, `runner_review_integration_test.go`) — каждый E2E тест = 100-200 строк.

2. **Нет table-driven structure.** CLAUDE.md: "Table-driven by default." 11 standalone scenarios нарушают конвенцию проекта. Общий setup (config, mock wiring, temp dirs) дублируется 11 раз.

3. **Cross-language test (M12) = 5 sub-scenarios** внутри одного Scenario. Go, Python, JS/TS, Java, mixed — это 5 отдельных test cases с разными file structures. "Scenario: FINAL — cross-language scope hints" — слишком широкий.

4. **Build tag `//go:build integration`** — правильно. Но нет стратегии для CI timeout. 11 E2E tests с MockClaude = 11 subprocess spawns. На CI это может быть 30-60 секунд.

**Рекомендация:**
- Разбить на 3 test файла: `runner_final_knowledge_test.go` (6.2 injection), `runner_final_distill_test.go` (6.5 distillation), `runner_final_e2e_test.go` (6.8 full flow)
- Table-driven: scenarios с общей структурой (config variations) → table; unique scenarios → standalone
- Cross-language: отдельная table с `projectFiles map[string]string` для каждого языка

---

### [A3-HIGH-2] ralph-index.md — redundant, потенциально вредный

**Severity: HIGH**

Story 6.5b AC "Index file auto-generation": `ralph-index.md` — auto-generated TOC всех ralph-*.md файлов.

**Research finding:** Claude Code АВТОМАТИЧЕСКИ загружает ВСЕ файлы из `.claude/rules/`. Правила загрузки:
- Файлы без `paths` frontmatter — загружаются ВСЕГДА (unconditionally)
- Файлы с `paths` YAML frontmatter — загружаются по glob match

ralph-index.md будет загружен Claude Code как ещё один rules файл. Это:
1. **Бесполезный контекст** — Claude Code уже видит все ralph-*.md, index не добавляет информации
2. **Потребляет tokens** — markdown таблица с файлами, counts, dates = ~100-200 tokens
3. **Дублирование** — Claude Code's own mechanism уже индексирует rules
4. **Требует обновления** — при каждой дистилляции, при каждом изменении ralph-*.md

**Конкурентный контекст:** Ни Cursor, ни Continue, ни Aider не имеют index файлов для rules. Индексация встроена в инструмент.

Проект bmad-ralph сам использует `.claude/rules/go-testing-patterns.md` как index (7 topic files, ~122 patterns). Но это РУЧНОЙ файл с семантическим описанием. Auto-generated TOC — другое.

**Рекомендация:** Убрать ralph-index.md. Если нужна metadata для человека — `ralph status` CLI команда (Growth phase), не файл в rules/.

---

### [A3-HIGH-3] ValidateLearnings — JIT citation через os.Stat на NTFS/WSL = performance risk

**Severity: HIGH**

Story 6.2 AC: "ValidateLearnings(projectRoot, content) — os.Stat file existence check only."
Technical notes: "O(N) stat calls, ~50 entries × ~1ms = 50ms (negligible)."

**WSL/NTFS context** (из `.claude/rules/wsl-ntfs.md`): NTFS через WSL имеет значительно более высокую latency для file operations. `os.Stat` на WSL/NTFS = 5-15ms per call (не 1ms). 50 entries × 10ms = 500ms. При 200+ entries = 2+ секунды.

Это вызывается 3 раза за task (initial, retry, review) по shared `buildKnowledgeReplacements`. Итого: 1.5-6 секунд на stat calls per task.

**Рекомендация:** Cache stat results per session (map[string]bool). Файлы не меняются между execute/review calls в рамках одного task.

```go
type citationCache struct {
    results map[string]bool // path -> exists
}

func (c *citationCache) fileExists(path string) bool {
    if v, ok := c.results[path]; ok {
        return v
    }
    _, err := os.Stat(path)
    exists := err == nil
    c.results[path] = exists
    return exists
}
```

---

### [A3-MEDIUM-1] 2-stage template — правильная архитектура, но Stage 2 ordering = latent bug

**Severity: MEDIUM**

`config/prompt.go:73-84` сортирует ключи замен по алфавиту и применяет `strings.ReplaceAll` последовательно. Это означает:

1. Если placeholder A содержит text, который является placeholder B → double replacement. Пример: если `__FINDINGS__` заменяется на текст, содержащий `__LEARNINGS_CONTENT__` — вторая замена произойдёт.

2. Doc comment (строка 53) говорит: "replacement values MUST NOT contain other replacement placeholders". Это правильное ограничение, но оно enforce'd только документацией, не кодом.

**Risk assessment:** LOW в текущей архитектуре (user content не содержит `__UPPERCASE__` patterns). Но при добавлении LEARNINGS.md injection — user-written entries МОГУТ содержать `__PLACEHOLDER__` text (если пользователь пишет правила о placeholder'ах).

**Рекомендация:** Оставить как-is для v1. Добавить debug assertion в тестах: verify no replacement value contains any replacement key.

---

### [A3-MEDIUM-2] Cross-project universality — Go-centric examples в AC, но language-agnostic claims

**Severity: MEDIUM**

Story 6.2 claims "self-review content is generic (no language-specific assumptions)". Но:

1. Story 6.1 entry format: `## category: topic [source, file:line]` — `file:line` reference. Python tracebacks use `File "path", line N`. Java: `at Class.method(File.java:N)`. Format assumes `file:N` convention.

2. Story 6.5b scope hints: "Go scans top 2 levels, collects file extensions, maps to known language globs". Таблица маппинга не определена. Какие extensions → какие globs? `.py` → `*.py`? А `.pyx` (Cython)? `.jsx`/`.tsx`?

3. Story 6.8 Scenario FINAL — cross-language: "Go, Python, JS/TS, Java, mixed" — 5 языков. Но нет Ruby, Rust, C/C++, Swift, Kotlin. Canonical categories: testing, errors, config, cli, architecture, performance, security. Для Python "testing" = pytest conventions. Для Java = JUnit. Categories одинаковые, содержание разное.

**Research:** Aider решает эту проблему через tree-sitter (40+ languages auto-detected). Cursor через auto-detection framework. bmad-ralph не имеет parser для project type detection.

**Рекомендация:**
- Определить explicit extension → glob mapping table в config (не hardcode)
- Citation format: accept multiple formats (`file:line`, `File "path" line N`, etc.)
- Cross-language tests: начать с 4 (Go, Python, JS, Java), добавлять по мере feedback
- Canonical categories: language-agnostic by design (уже так), но examples в дистилляции должны быть language-aware

---

### [A3-MEDIUM-3] buildKnowledgeReplacements — 3 вызова, но нет caching

**Severity: MEDIUM**

Story 6.2 H7: "All 3 AssemblePrompt call sites use same shared function." Но `buildKnowledgeReplacements(projectRoot)` делает:
1. `os.ReadFile(LEARNINGS.md)`
2. `ValidateLearnings()` (N stat calls)
3. `filepath.Glob(ralph-*.md)` + N ReadFile calls
4. Concatenation + formatting

Это выполняется 3 раза per task (initial execute, retry execute, review). Файлы не меняются между вызовами (кроме LEARNINGS.md после review writes). 2 из 3 вызовов гарантированно читают идентичные данные.

**Рекомендация:** Cache at Runner level per task iteration. Invalidate after review session (which may write LEARNINGS.md).

---

## 2. Альтернативные архитектуры

### Текущая архитектура (v5)

```
┌─────────────────────────────────────────────────────┐
│                    runner.go                          │
│                                                       │
│  buildKnowledgeReplacements(projectRoot)              │
│  ┌──────────────────────────────────────────────┐     │
│  │ 1. ReadFile(LEARNINGS.md)                     │     │
│  │ 2. ValidateLearnings() → stat N files         │     │
│  │ 3. Glob(ralph-*.md) → ReadFile each           │     │
│  │ 4. Return map{__LEARNINGS__: ..., __RALPH__: } │     │
│  └──────────────────────────────────────────────┘     │
│                        │                              │
│                        ▼                              │
│  AssemblePrompt(template, TemplateData, replacements) │
│  ┌──────────────────────────────────────────────┐     │
│  │ Stage 1: text/template (bool conditionals)    │     │
│  │ Stage 2: strings.ReplaceAll (content inject)  │     │
│  │ NO BUDGET CHECK                               │     │
│  └──────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────┘
```

### Альтернатива A: Budget-Aware Assembly

```
┌──────────────────────────────────────────────────────┐
│                    runner.go                           │
│                                                        │
│  knowledgeCache := NewKnowledgeCache(projectRoot)     │
│  ┌───────────────────────────────────────────────┐    │
│  │ Lazy load + cache per task:                    │    │
│  │ - LEARNINGS.md (invalidate after review)       │    │
│  │ - ralph-*.md (stable within session)           │    │
│  │ - citation stat cache (reset per task)         │    │
│  └───────────────────────────────────────────────┘    │
│                        │                               │
│                        ▼                               │
│  BudgetedReplacements(cache, budget int) map[string]  │
│  ┌───────────────────────────────────────────────┐    │
│  │ 1. Compute all replacements                    │    │
│  │ 2. Sum total chars                             │    │
│  │ 3. If over budget:                             │    │
│  │    a) Truncate LEARNINGS to last N entries     │    │
│  │    b) Exclude ralph-misc.md if still over      │    │
│  │    c) Log degradation warning                  │    │
│  └───────────────────────────────────────────────┘    │
│                        │                               │
│                        ▼                               │
│  AssemblePrompt (unchanged — Stage 1 + Stage 2)       │
└──────────────────────────────────────────────────────┘
```

**Преимущества:** Budget control в caller (SRP), caching, graceful degradation.
**Недостатки:** Ещё один слой абстракции. Для v1 может быть premature.

### Альтернатива B: Progressive Disclosure (Aider-like)

```
┌────────────────────────────────────────────────────────┐
│ Knowledge Loading Strategy                              │
│                                                          │
│ Priority 1 (always): ralph-critical.md (T1, ~20 lines) │
│ Priority 2 (always): ralph-{scope-matched}.md           │
│ Priority 3 (budget permitting): LEARNINGS.md reversed   │
│ Priority 4 (if space): ralph-misc.md                    │
│                                                          │
│ Budget: max(budget - other_content, 2000 chars)         │
│ Strategy: fill until budget exhausted, skip rest        │
└────────────────────────────────────────────────────────┘
```

**Преимущества:** Аналогична Aider's dynamic map budget. Самые важные правила всегда присутствуют.
**Недостатки:** Сложность приоритизации. Нужен ranking механизм.

### Рекомендация: Альтернатива A для v1

Альтернатива A добавляет minimal complexity (cache + budget check) при maximum safety. Альтернатива B — Growth phase, когда данные о usage patterns доступны.

---

## 3. Research Recommendations

### R1: MCP Config Standardization — Watch & Adapt

MCP config format конвергирует на JSON с `mcpServers` ключом, но НЕ стандартизирован формально. `gleanwork/mcp-config-schema` — community проект для TypeScript type-safe builders. Go эквивалента нет.

**Рекомендация:** Defensive parsing с `json.RawMessage`. Не привязываться к конкретной schema. Test with Claude Code's actual settings.json format. Подготовить migration path если формат изменится.

### R2: Context Window Budgeting — Adopt HumanLayer's FIC

HumanLayer Advanced Context Engineering (ACE): target 40-60% context utilization. Для Claude Code с 200K window — это 80-120K tokens. bmad-ralph's prompt = 2-5K tokens base + injected content. Budget for knowledge = ~2-3K tokens (safe).

**Конкретная метрика:** LEARNINGS.md injection ≤ 100 строк (~2K tokens). ralph-*.md ≤ 150 строк (~3K tokens). Total knowledge ≤ 5K tokens (~2.5% of 200K window).

### R3: Observation Masking для Session Logs

JetBrains research (2025): observation masking = halves cost, matches solve rate. Для bmad-ralph: session output (Claude's responses) не нужно хранить полностью. Store only: commit hash, files changed, test results, errors. Не хранить raw Claude output.

### R4: Agent README Quality > Quantity

arXiv 2602.11988: poorly written context files REDUCE task success by >20%. Для bmad-ralph: knowledge quality gates (Story 6.1) КРИТИЧНЫ. Без effective quality control — knowledge system вредит больше чем помогает.

### R5: Tree-sitter для Project Detection (Growth)

Aider использует tree-sitter для 40+ languages. Go binding: `github.com/smacker/go-tree-sitter`. Для scope hints (M4) — tree-sitter определит project type точнее чем file extension scan.

**Не рекомендую для v1** — новая зависимость, CGO issues. Но для Growth — приоритетный путь.

---

## 4. Подтверждённое — что правильно

### P1: 2-stage template architecture — CORRECT

Разделение Stage 1 (Go template, code-controlled booleans) и Stage 2 (string replace, user content) — архитектурно правильное. Защита от template injection: user content никогда не проходит через `text/template`. Это лучше чем:
- Единый template engine (Jinja2 approach) — security risk
- Pure string replacement (no conditionals)
- Multiple template engines

Zero dependencies (stdlib only) — matches project constraint (3 deps max).

### P2: .claude/rules/ для distilled knowledge — CORRECT

Использование native Claude Code infrastructure (.claude/rules/ с YAML frontmatter) — лучшее доступное решение. Не изобретает собственный injection mechanism. Progressive disclosure через globs.

Research подтверждает: Claude Code загружает rules автоматически с scope matching. Это БЕСПЛАТНАЯ инфраструктура — Go код не нужен для injection distilled rules. Нужен только для LEARNINGS.md (hot buffer).

### P3: Serena = MCP, not CLI (C3) — CORRECT

Research подтверждает: Serena — MCP server. Все инструменты (Claude Code, Cursor, Windsurf) используют MCP через config files, не через CLI exec. Minimal interface (2 methods) — YAGNI-correct.

### P4: MonotonicTaskCounter (H1) — CORRECT

Persisted counter для cooldown — правильный подход. Cross-session state через JSON file. Simple, testable, no complex state machine.

### P5: BEGIN/END markers protocol (H6) — CORRECT

Standard LLM structured output pattern. Research подтверждает: markers + structured sections — best practice для parsing LLM output. Claude's instruction-following для markers = 95%+ compliance.

### P6: buildKnowledgeReplacements shared function (H7) — CORRECT

DRY для 3 call sites. Правильный threshold для extraction. Signatura `(projectRoot string) (map[string]string, error)` — чистая, тестируемая.

### P7: HasLearnings conditional self-review (H3) — CORRECT

Research (Live-SWE-agent): single reflection prompt = +12% quality. Conditional on HasLearnings — no overhead when no knowledge. Zero-cost when disabled.

### P8: No CLAUDE.md modification — CORRECT

FR26 через невмешательство. CVE-2025-59536, CVE-2026-21852 подтверждают: programmatic config editing = confirmed risk class. Правильное архитектурное решение.

---

## 5. Итоговые рекомендации (приоритизированные)

| # | Severity | Рекомендация | Effort |
|---|----------|-------------|--------|
| 1 | CRITICAL | Budget check для prompt assembly (caller-side) | S |
| 2 | CRITICAL | MCP config parsing — только `.claude/settings.json`, defensive JSON | S |
| 3 | HIGH | ralph-index.md — убрать, Claude Code уже индексирует | XS |
| 4 | HIGH | Integration tests — split на 3 файла, table-driven where possible | M |
| 5 | HIGH | ValidateLearnings stat cache для WSL/NTFS performance | S |
| 6 | MEDIUM | Cross-language extension→glob mapping — config table, не hardcode | S |
| 7 | MEDIUM | buildKnowledgeReplacements caching per task iteration | S |
| 8 | LOW | Stage 2 replacement ordering — test assertion for no cross-references | XS |

**Общая оценка:** Архитектура 2-stage template + .claude/rules/ injection — ПРАВИЛЬНАЯ и подтверждена research. Основные проблемы: отсутствие budget control (CRITICAL), хрупкость MCP config parsing (CRITICAL), и over-engineering ralph-index.md (HIGH). Stories 6.2 и 6.7 реализуемы с указанными корректировками. Story 6.8 требует структурной реорганизации тестов.
