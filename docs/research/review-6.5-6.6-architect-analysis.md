# Архитектурный анализ Stories 6.5, 6.6 — Предварительный

**Автор:** architect-distillation
**Дата:** 2026-02-28
**Статус:** Черновик (ожидает cross-review с аналитиком)

---

## 1. Story 6.5: Budget Enforcement & Auto-Distillation

### 1.1. Trigger Point — КОРРЕКТЕН

AC: "AFTER gate check, BEFORE next iteration".
В runner.go Execute() текущий flow после clean review:
```
clean review → completedTasks++ → gate check (approve/skip/quit/retry) → continue
```
Distillation вставляется ПОСЛЕ gate check (approve/skip fall through) и ПЕРЕД `continue` к следующей итерации. Архитектурно корректно: задача полностью завершена, gate пройден, можно безопасно distill.

**Concern A1 (MEDIUM):** При gate retry (revert task → re-execute) distillation НЕ запускается — правильно. Но при gate skip distillation тоже НЕ запускается (wasSkipped=true, continue в строке 577). Это правильное поведение? Если задача skipped, но LEARNINGS.md уже >150 строк — distillation откладывается до следующей clean review. Не баг, но стоит документировать.

### 1.2. Circuit Breaker — 3 ПРОБЛЕМЫ

**Concern A2 (HIGH): Определение "failure" не специфицировано.**
AC говорит "auto-distillation failed 3 times consecutively". Но что считается failure?
- `claude -p` вернул non-zero exit code?
- ValidateDistillation отвергла output?
- Ошибка записи файлов?
- Все три?

Рекомендация: определить failure как "ValidateDistillation rejected OR claude -p exit != 0 OR any I/O error during file write". Все три — failure для circuit breaker.

**Concern A3 (HIGH): Task counting для HALF-OPEN probe несовместим с ephemeral counter.**
AC: "10 tasks elapsed since last failure". Runner.completedTasks — ephemeral (сбрасывается при restart). DistillState.LastDistillTask — persisted. Но это разные counting domains:
- completedTasks считает задачи в ТЕКУЩЕМ запуске ralph run
- LastDistillTask хранит timestamp/count последней distillation

Если ralph run завершается после 3 задач, при следующем запуске completedTasks=0. Как сравнить с LastDistillTask?

Решение: DistillState должен хранить TasksSinceLastAttempt (инкрементируется при каждом clean review, сбрасывается при distillation attempt). Или использовать абсолютный cross-session counter.

**Concern A4 (MEDIUM): Permanent OPEN state без escape.**
Circuit breaker OPEN → 10 tasks → HALF-OPEN → probe fails → OPEN (reset counter).
Если root cause не в LLM output, а в corrupt LEARNINGS.md или broken file system — probe будет вечно fail. Единственный escape — `ralph distill` (Story 6.6). Но пользователь может не знать о circuit breaker state.

Рекомендация: логировать "Circuit breaker OPEN for N tasks — consider `ralph distill`" при каждом skipped distillation.

### 1.3. Cooldown — 1 ПРОБЛЕМА

**Concern A5 (MEDIUM): Cross-session vs within-session ambiguity.**
AC: "≥5 tasks since last distillation". Та же проблема что A3 — completedTasks ephemeral.
DistillState.LastDistillTask нужен как absolute counter, не зависящий от ralph run lifecycle.

Решение: DistillState получает собственный MonotonicTaskCounter, инкрементируемый при каждом clean review (persisted to disk). Cooldown сравнивает MonotonicTaskCounter - LastDistillTask >= 5.

### 1.4. Multi-file Output — 3 ПРОБЛЕМЫ

**Concern A6 (HIGH): LLM-generated glob patterns не валидируются.**
AC: "scope hints auto-detected from project file types (NOT hardcoded to any language)".
Distillation prompt инструктирует LLM выбрать glob patterns. Но LLM может вернуть:
- Невалидный glob: `*_test.{go` (missing closing brace)
- Слишком широкий glob: `**/*` (matches everything — defeats purpose)
- Несуществующий pattern: `*.rs` для Go проекта

Рекомендация: Go-side validation после parsing:
1. `filepath.Match` с dummy path для syntax check
2. Glob pattern must match ≥1 file в проекте (`filepath.Glob`)
3. Fallback: если invalid → use `**` (always loaded)

**Concern A7 (MEDIUM): Minimum 3 rules per file — merge into ralph-misc.md может создать monster file.**
Если 10 категорий × 2 rules each → all merge into ralph-misc.md = 20 rules. Один файл с 20 rules и globs: ["**"] — эффективно монолит.

Рекомендация: ralph-misc.md НЕ должен иметь globs: ["**"]. Лучше: no frontmatter (loaded only by ralph prompt injection, not by Claude Code auto-load).

**Concern A8 (MEDIUM): Frequency tracking `[freq:N]` зависит от LLM counting.**
AC: "auto-promote categories with ≥6 entries" и "promote [freq:N≥10] entries".
LLM инкрементирует freq:N. Но LLMs плохо считают и инкрементируют числа.

Рекомендация: Go code should parse [freq:N] and increment, не LLM. При distillation input подготовка: Go парсит freq, инкрементирует за merged duplicates, передаёт в prompt. Output validation: Go проверяет freq consistency.

### 1.5. Post-validation (ValidateDistillation) — 2 ПРОБЛЕМЫ

**Concern A9 (HIGH): Criterion #3 "Recent entries (last 3 sessions)" — нет session markers в формате.**
Формат записи: `## category: topic [source, file:line]`. Нет session ID, нет timestamp.
Как определить "last 3 sessions"?

Варианты:
a) LEARNINGS.md append-only → last N entries = most recent. Criterion → "last 20 entries preserved" (approximate 3 sessions × ~7 entries each).
b) Add session marker: `## category: topic [source, file:line, session:abc123]`
c) Add timestamp: `## category: topic [source, file:line, 2026-02-28]`

Рекомендация: вариант (a) — самый простой, не требует формата изменений. "Last 20% of entries must be preserved" вместо "last 3 sessions".

**Concern A10 (MEDIUM): Criterion #4 "Citation preservation ≥ 80%" — parsing complexity.**
Нужно: извлечь все citations из old content, из new content, посчитать intersection.
Citation format: `[source, file:line]` — нужен regexp parser.
Edge cases: merged entries имеют multiple citations ("both citations preserved").

Рекомендация: implement as regexp-based parser. Допустимая approximation: count unique file paths, не exact line numbers (lines shift after refactoring).

### 1.6. No-FIFO Decision — CRITICAL CONCERN

**Concern A11 (CRITICAL): Unbounded growth при stuck circuit breaker.**
Сценарий:
1. LEARNINGS.md reaches 150 lines → auto-distill triggered
2. Distillation fails 3x → circuit breaker OPEN
3. Development continues, reviews write more lessons
4. LEARNINGS.md grows to 300, 500, 1000 lines
5. No FIFO, no archive, no forced truncation

AC: "300+ lines = 3-4% context (200K window), linear decay not cliff".

**Проблема:** context window percentage — неправильная метрика. Правильная метрика — instruction count для compliance. Research [S25]: ~15 rules = 94%, 125+ rules = ~40-50%. При 500 lines LEARNINGS.md = ~150-200 distinct rules, FAR beyond compliance threshold.

**Вторая проблема:** injecting 500+ lines of potentially stale knowledge degrades performance для ALL tasks, не только для knowledge retrieval.

**Рекомендация:** добавить HARD cap (не в Story 6.5, а как safety net):
- При Lines > 2× Limit (400 lines по дефолту): log WARNING "LEARNINGS.md exceeds 2x budget — knowledge injection degraded, manual `ralph distill` recommended"
- При Lines > 3× Limit (600 lines): STOP injecting into prompts (empty __LEARNINGS_CONTENT__). Still write new entries, but don't inject old ones. Self-healing: next successful distillation brings it under limit.

Это НЕ FIFO, НЕ archive, НЕ forced truncation. Это "circuit breaker for injection" — отдельный от distillation circuit breaker.

### 1.7. Concurrency — POTENTIAL ISSUE

**Concern A12 (LOW для auto, MEDIUM для manual):**
Auto-distillation runs sequentially в runner loop — нет concurrency issue.
Но `ralph distill` (Story 6.6) может запуститься параллельно с `ralph run`.

Если ralph run пишет LEARNINGS.md (через review session) ОДНОВРЕМЕННО с ralph distill:
1. ralph distill читает LEARNINGS.md (150 lines)
2. ralph run's review session записывает новый lesson (151 lines)
3. ralph distill завершает, записывает distilled output (80 lines)
4. Lesson из шага 2 — ПОТЕРЯН

Рекомендация: file lock (flock) при distillation write, или advisory note в docs.

### 1.8. DistillState Persistence — 1 ПРОБЛЕМА

**Concern A13 (MEDIUM): Corrupt or missing .state file.**
`LEARNINGS.md.state` (JSON) — что при corrupt JSON? Parse error → default state?

Рекомендация: on parse error → log warning, use default state (CLOSED, FailCount=0). Документировать в AC.

---

## 2. Story 6.6: Distillation CLI — ralph distill

### 2.1. Архитектура — КОРРЕКТНА

Простой Cobra subcommand → reuses AutoDistill() + ValidateDistillation() из Story 6.5.
CB reset на успех — правильный escape hatch.

### 2.2. Missing Features — 2 РЕКОМЕНДАЦИИ

**Concern B1 (LOW): Нет --dry-run.**
`ralph distill --dry-run` показал бы: what would be distilled, how many categories, what scope hints — без записи. Ценно для debugging и verification.

Не блокирует, но стоит добавить как TODO.

**Concern B2 (LOW): Нет --force для bypass validation.**
При debugging: distillation output rejected by validation, но пользователь знает что output OK.
`ralph distill --force` bypasses ValidateDistillation.

Не блокирует, но стоит добавить как TODO.

---

## 3. Cross-cutting Concerns

### 3.1. Output Parsing из claude -p

**Concern C1 (HIGH): Формат distillation output не специфицирован как machine-parseable.**
AC: "output parsed by Go code" с `## CATEGORY: <name>` sections.
Но LLM может:
- Добавить preamble/explanation перед sections
- Использовать different header format ("## Category — Testing" vs "## CATEGORY: testing")
- Включить markdown formatting внутри sections

Рекомендация: distillation prompt MUST include exact output format specification с markers:
```
BEGIN_DISTILLED_OUTPUT
## CATEGORY: testing
...rules...
## CATEGORY: errors
...rules...
END_DISTILLED_OUTPUT
```
Go parser ищет markers, ignoring preamble/postamble. Это standard pattern для LLM structured output.

### 3.2. Token Cost Analysis

Story 6.5: ~8K tokens per distillation. При 100 задачах и cooldown 5: max 20 distillations = ~160K tokens. На самом деле значительно меньше (distillation only когда >150 lines). Cost negligible — ПОДТВЕРЖДАЮ.

### 3.3. Universality

Ralph — UNIVERSAL CLI tool. Distillation prompt генерирует scope hints для ANY language.
LLM capability bet: Claude хорошо знает file patterns для Go, Python, JS, Rust, Java. Менее уверен для Haskell, Elixir, Zig. Но fallback ("**") safe.

---

## 4. Summary: Finding Severity

| # | Concern | Severity | Story |
|---|---------|----------|-------|
| A2 | Failure definition ambiguous | HIGH | 6.5 |
| A3 | Ephemeral task counter | HIGH | 6.5 |
| A6 | Unvalidated glob patterns | HIGH | 6.5 |
| A9 | No session markers for recency | HIGH | 6.5 |
| A11 | Unbounded growth no-FIFO | CRITICAL | 6.5 |
| C1 | Output format not machine-parseable | HIGH | 6.5 |
| A1 | Skip doesn't trigger distill | MEDIUM | 6.5 |
| A4 | Permanent OPEN no escape warning | MEDIUM | 6.5 |
| A5 | Cooldown cross-session ambiguity | MEDIUM | 6.5 |
| A7 | ralph-misc.md monster file | MEDIUM | 6.5 |
| A8 | LLM freq counting unreliable | MEDIUM | 6.5 |
| A10 | Citation parsing complexity | MEDIUM | 6.5 |
| A12 | Concurrent distill+run race | MEDIUM | 6.6 |
| A13 | Corrupt .state file handling | MEDIUM | 6.5 |
| B1 | No --dry-run | LOW | 6.6 |
| B2 | No --force | LOW | 6.6 |

**Total: 1 CRITICAL, 5 HIGH, 8 MEDIUM, 2 LOW = 16 concerns**

---

## 5. Дополнительные findings из Deep Research (2025-2026)

### 5.1. LLM Scaling Paradox (arxiv.org/abs/2602.09789, Feb 2026)

**CRITICAL для Story 6.5:** Larger compressor LLMs can *underperform* smaller ones during context compression. Two failure modes:
1. **Knowledge overwriting**: source facts replaced by model's own priors (hallucinated entities)
2. **Semantic drift**: causal relationships distorted ("A causes B" → "B causes A")

Это ПРЯМАЯ угроза для `claude -p` distillation. Opus/Sonnet при compression может "додумать" правила, которых не было в оригинале.

**Рекомендация усиливает A11:** ValidateDistillation criterion #4 (citation ≥80%) — необходим но не достаточен. Добавить: "no new citations may appear that weren't in input" (prevents hallucinated rules).

### 5.2. Model Collapse (Nature 2024, Shumailov et al.)

Mathematical inevitability: variance grows as σ²(1 + n/M). Distillation loop = self-referencing.
- **Early collapse**: loss of minority/edge-case patterns (insidious — overall quality *appears* to improve)
- **Late collapse**: distribution converges to nothing resembling original

**Mitigation already in design:** "never remove entries from last 3 sessions" = anchor set. Но нужно формализовать: при distillation input, MARK anchor entries explicitly → distillation prompt MUST NOT remove marked entries.

### 5.3. Atomic File Writes на WSL/NTFS

`os.Rename` НЕ атомарен на Windows. natefinch/atomic wraps Windows MoveFileEx API.

Но bmad-ralph's backup strategy (write .bak → overwrite → restore on failure) NOT atomic across multiple files. Acceptable: worst case = partial write, но backup .bak files позволяют manual recovery.

**Рекомендация:** document recovery procedure: "If ralph crashes during distillation, .bak files in project root can be renamed to restore previous state."

### 5.4. Go Circuit Breaker Libraries

mercari/go-circuitbreaker v1.12.3 — `FailOnContextCancel(false)` prevents LLM timeout from being counted as failure. Но для bmad-ralph custom implementation preferred (file-persisted state, task-count-based not time-based).

### 5.5. ChatGPT-4 vs Zlib: 23% Worse for Semantic Preservation

Quantifies the risk: LLM compression loses ~23% more semantic content than lossless compression. This is why ValidateDistillation with 7 criteria is essential — NOT optional optimization.
