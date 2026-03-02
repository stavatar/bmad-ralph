# Совместный критический обзор Stories 6.5, 6.6 — Auto-Distillation & CLI

**Авторы:** architect-distillation + analyst-distillation
**Дата:** 2026-02-28
**Статус:** FINAL CONSENSUS (подтверждён обоими участниками 2026-02-28)

---

## Методология

- Прочитано: Epic 6 (8 stories, 59 AC), 3 исследовательских отчёта (R1-R3, 62 источника)
- Изучен source code: runner/runner.go (764 строки), runner/knowledge.go, config/config.go, config/prompt.go, config/defaults.yaml, session/session.go, runner/prompts/execute.md, runner/prompts/review.md
- Deep Research: knowledge compression 2025-2026, circuit breaker patterns, multi-file management, distillation validation, model collapse
- Независимый анализ каждым участником → cross-review → consensus

---

## Verdict

**Архитектура Stories 6.5/6.6 в целом КОРРЕКТНА.** 3-layer model (Go semantic dedup → LLM distillation → Go post-validation) хорошо обоснована тремя раундами исследований. Circuit breaker, cooldown, backup/restore — правильные safety nets.

**НО: 14 concerns требуют внимания перед реализацией** (1 CRITICAL, 4 HIGH, 7 MEDIUM, 2 LOW). Два concerns потенциально меняют дизайн: unbounded growth без injection protection (J1) и non-deterministic category naming (J3).

---

## CRITICAL

### J1. Unbounded LEARNINGS.md growth при stuck circuit breaker

**Источник:** architect A11 + analyst Issue 1 (consensus severity: HIGH→CRITICAL с mitigation)

**Проблема:** AC утверждает "300+ строк = 3-4% контекста (200K окно), linear decay не cliff". Это ВВОДИТ В ЗАБЛУЖДЕНИЕ:
- 200K — полный window. Реальный budget для user content ≈70-100K (system prompt, managed policy, CLAUDE.md, rules, tools уже занимают 30-50%).
- SFEIR research [R2, S25]: ~15 правил = 94% compliance. 300+ строк = 60-100+ правил при 3-5 строках/правило — well beyond compliance threshold.
- Context rot [R1, S5]: 30-50% degradation при полном контексте. Каждые 50 строк — дополнительный context dilution.
- Сценарий: CB OPEN → development continues → LEARNINGS.md = 500-1000 строк → injection degrades ALL tasks.
- LLM Scaling Paradox [arxiv.org/abs/2602.09789, Feb 2026]: compressor LLMs can overwrite source facts with own priors — compounding risk at large file sizes.

**Consensus recommendation (architect proposal, analyst agreed on compromise):**
1. **Soft warning** при Lines > 2× Limit (400): log "LEARNINGS.md exceeds 2x budget — quality degradation likely, consider `ralph distill`"
2. **Injection circuit breaker** при Lines > 3× Limit (600): STOP injecting в промпты (empty `__LEARNINGS_CONTENT__`). Запись новых entries продолжается (zero knowledge loss). Следующая успешная distillation автоматически восстанавливает injection.
3. Это НЕ FIFO, НЕ archive, НЕ truncation. Это **automatic degradation of injection при overflow** — self-healing без human intervention.

**Impact на AC:** Добавить scenario "Injection disabled at 3x budget" в Story 6.5.

---

## HIGH

### J2. Non-deterministic category naming в multi-file output

**Источник:** analyst Issue 3 + architect C1 (consensus severity: HIGH)

**Проблема:** Distillation prompt инструктирует Claude сгруппировать по категориям. Go code парсит `## CATEGORY: <name>`. Но:
- Claude может назвать category "testing", "tests", "test-patterns" — каждый раз по-разному.
- При повторных distillations: category drift → ralph-testing.md + ralph-test-patterns.md → duplicate files, knowledge fragmentation.
- Scope hints тоже LLM-зависимы: `*.test.go` vs `*_test.go` — одна ошибка = правила не загружаются Claude Code.
- LLM output format не специфицирован с machine-parseable markers — preamble/explanations будут сломать Go parser.

**Consensus recommendation:**
1. **Canonical category list** в Go коде: `testing, errors, config, cli, architecture, performance, misc`. Distillation prompt outputs ONLY these. Go code rejects unknown → merge into `misc`.
2. **BEGIN/END markers** в distillation prompt: `BEGIN_DISTILLED_OUTPUT` / `END_DISTILLED_OUTPUT`. Go parser ignores preamble/postamble.
3. **Go-side glob validation:** `filepath.Match` syntax check + `filepath.Glob` ≥1 match in project. Invalid glob → fallback `**`.
4. **Pre-distillation cleanup:** удаление ВСЕХ старых ralph-*.md (с backup) перед записью новых.

**Impact на AC:** Добавить canonical categories, output markers, glob validation. Значительное изменение distillation prompt design.

### J3. Circuit breaker: ephemeral task counter несовместим с persisted state

**Источник:** architect A3 + analyst Issue 2c (consensus severity: HIGH)

**Проблема:** AC сравнивает `completedTasks` (in-memory, resets on restart) с `DistillState.LastDistillTask` (persisted). При restart: completedTasks=0, LastDistillTask=25 → cooldown НИКОГДА не пройдёт в первом run. Для HALF-OPEN probe: "10 tasks elapsed" — elapsed от какого счётчика?

**Consensus recommendation:** DistillState получает `MonotonicTaskCounter` (persisted, инкрементируется при каждом clean review). Cooldown: `MonotonicTaskCounter - LastDistillTask >= 5`. HALF-OPEN: `MonotonicTaskCounter - LastFailureTask >= 10`.

**Impact на AC:** Переписать cooldown и half-open AC с использованием persisted counter.

### J4. Circuit breaker "failure" не определён

**Источник:** architect A2 (consensus severity: HIGH)

**Проблема:** AC говорит "auto-distillation failed 3 times consecutively". Не специфицировано: что считается failure?

**Consensus recommendation:** Failure = ANY of:
- `claude -p` вернул non-zero exit code
- ValidateDistillation rejected output
- I/O error при записи файлов
- `claude -p` timeout (>120s default)

**Impact на AC:** Добавить explicit failure definition.

### J5. ValidateDistillation: "last 3 sessions" — нет session markers

**Источник:** architect A9 + analyst Issue 6 (consensus severity: HIGH)

**Проблема:** Criterion #3: "Recent entries (last 3 sessions) preserved". Формат записи `## category: topic [source, file:line]` НЕ содержит session ID или timestamp. Невозможно определить "last 3 sessions".

**Consensus recommendation:** Заменить "last 3 sessions" на "last 20% of entries" (append-only → tail = most recent). Простое, не требует изменения формата.

**Impact на AC:** Переформулировать criterion #3 в ValidateDistillation.

---

## MEDIUM

### J6. Frequency tracking `[freq:N]` ненадёжен при LLM counting

**Источник:** architect A8 + analyst Issue 4 (consensus)

**Проблема:** LLMs плохо считают и инкрементируют числа. `[freq:N]` инкрементируется distillation prompt.

**Recommendation:** Go-side post-validation: parse `[freq:N]` в output, проверить что N ≥ N в input (monotonic — never decreases). Go code handles increments при WriteLessons (semantic dedup merge counts).

### J7. Circuit breaker: 4 дополнительных gap'а

**Источник:** analyst Issue 2a,2b,2d + architect A4,A13 (consensus)

**Recommendations:**
- **2a/A4 (no max-open-time):** Добавить time-based half-open: `min(10 tasks, 72 hours)`. Log "CB OPEN for N tasks — consider `ralph distill`" при каждом skip.
- **2b/A13 (state file corruption):** Parse error → default CLOSED (fail-open). Log warning.
- **2d (no CB metrics):** Добавить `transition_count` и `last_transition_time` в DistillState.

### J8. ValidateDistillation: 2 хрупких критерия

**Источник:** analyst Issue 6 + architect A10 (consensus)

**Recommendations:**
- **Criterion #4 (citation ≥80%):** Normalize paths (basename) before comparison. Count unique files, not exact line numbers.
- **Criterion #6 (category ≥80%):** Lower to 60% OR track "merged" separately from "dropped". Valid merge ≠ category loss.
- **NEW criterion:** "No new citations may appear that weren't in input" — prevents hallucinated rules (LLM Scaling Paradox research).

### J9. Trigger point precision в Execute()

**Источник:** analyst Issue 9 + architect A1 (consensus)

**Recommendation:** Clarify in AC: distillation triggered ТОЛЬКО after clean review (not emergency skip). Insert AFTER gate check resolution (line 615 in current code), BEFORE continue to next iteration. Gate latency not impacted by distillation.

### J10. ralph-misc.md может стать монолитом

**Источник:** architect A7 (consensus)

**Recommendation:** ralph-misc.md НЕ получает `globs: ["**"]`. Loaded only через ralph prompt injection (`__RALPH_KNOWLEDGE__`), не через Claude Code auto-load. Предотвращает re-creation монолита.

### J11. Concurrent ralph distill + ralph run race condition

**Источник:** architect A12 (consensus)

**Recommendation:** Advisory note в `ralph distill --help`: "Do not run while `ralph run` is active." File lock (flock) — desirable но LOW priority (CLI tool, single user).

### J12. Config fields отсутствуют в спецификации

**Источник:** analyst Issue 7 (consensus)

**Recommendation:** Story 6.5 AC должен явно включить: "Config struct extended with DistillCooldown int, DistillTargetPct int, DistillMaxFailures int. defaults.yaml updated with distill_cooldown: 5, distill_target_pct: 50, distill_max_failures: 3."

---

## LOW

### J13. ralph distill: edge cases

**Источник:** analyst Issue 8 + architect B1,B2 (consensus)

**Recommendations:**
- Minimum threshold: if lines < 50, warn + require `--force`.
- CB reset: only если distilled file was ≥ soft threshold.
- `--dry-run` flag: run distill + validate + print summary, don't write. (TODO, не blocker)
- `--force` flag: bypass ValidateDistillation. (TODO, не blocker)

### J14. Backup single-generation + Model Collapse risk

**Источник:** analyst Issue 10 + architect deep research (consensus)

**Recommendations:**
- Backup: keep `.bak` and `.bak.1` (2-generation). Minimal complexity.
- Model Collapse mitigation: explicitly MARK anchor entries (last 20% of file) в distillation input. Prompt: "entries marked [ANCHOR] MUST NOT be removed".
- Document recovery: "If ralph crashes during distillation, .bak files can be renamed to restore."

---

## Metrics: Effectiveness

**Источник:** analyst Issue 5 (consensus)

Текущие metrics (volume: entries before/after, stale count, etc.) — необходимы но не достаточны. Они отвечают на "сколько", но не на "помогло ли".

**Minimum viable metric (Story 6.5):** Trend logging при каждой distillation: "entries: 160→95 (-41%), categories: 8→7, stale: 12 removed, T1: 2 promoted".

**Growth phase metrics (defer):**
- Correlation: review findings/task BEFORE vs AFTER knowledge injection
- Utilization: entries cited in execute sessions vs total entries

---

## Deep Research Insights (дополнительный контекст)

1. **LLM Scaling Paradox (Feb 2026):** Compressor LLMs can *overwrite* source facts with own priors. Two failure modes: knowledge overwriting, semantic drift. ValidateDistillation criterion "no new citations" directly addresses this.

2. **Model Collapse (Nature 2024):** Self-referencing distillation loop → mathematical variance growth σ²(1+n/M). Early collapse = loss of edge-case patterns (appears as quality improvement!). Anchor set (never remove recent entries) = proven mitigation.

3. **23% Semantic Loss (LLM vs Zlib):** ChatGPT-4 23% worse than lossless for semantic preservation. ValidateDistillation 7 criteria = essential, not optional.

4. **Circuit Breaker Best Practices (2025-2026):** mercari/go-circuitbreaker: `FailOnContextCancel(false)` for LLM services. natefinch/atomic for WSL/NTFS atomic writes. bmad-ralph correctly uses custom file-persisted CB (task-count-based, not time-based).

5. **Convergent Evolution:** MemGPT/Letta, MemOS, GitHub Copilot, claude-mem — all converge on tiered memory + compression + selective injection. bmad-ralph's 3-tier architecture aligns with industry consensus.

---

## Summary Table

| ID | Finding | Severity | Effort | Story |
|----|---------|----------|--------|-------|
| J1 | Unbounded growth + injection CB | CRITICAL | MEDIUM | 6.5 |
| J2 | Category name drift | HIGH | MEDIUM | 6.5 |
| J3 | Ephemeral task counter | HIGH | LOW | 6.5 |
| J4 | Failure definition missing | HIGH | LOW | 6.5 |
| J5 | No session markers for recency | HIGH | LOW | 6.5 |
| J6 | freq:N LLM counting | MEDIUM | LOW | 6.5 |
| J7 | CB additional gaps (4) | MEDIUM | LOW | 6.5 |
| J8 | Fragile validation criteria | MEDIUM | LOW | 6.5 |
| J9 | Trigger point precision | MEDIUM | LOW | 6.5 |
| J10 | ralph-misc.md monster file | MEDIUM | LOW | 6.5 |
| J11 | Concurrent distill+run race | MEDIUM | LOW | 6.6 |
| J12 | Config fields missing | MEDIUM | LOW | 6.5 |
| J13 | CLI edge cases | LOW | LOW | 6.6 |
| J14 | Backup + model collapse | LOW | LOW | 6.5 |

**Total: 1 CRITICAL, 4 HIGH, 7 MEDIUM, 2 LOW = 14 joint findings**

---

## Architectural Recommendations

1. **ОБЯЗАТЕЛЬНО до реализации:** Fix J1 (injection CB), J2 (canonical categories), J3 (persisted counter), J4 (failure definition), J5 (recency = last 20%).
2. **В процессе реализации:** J6-J12 как AC amendments.
3. **Growth phase:** Effectiveness correlation metrics, `--dry-run`, `--force`.
4. **Invariant:** 3-layer architecture КОРРЕКТНА. No FIFO decision КОРРЕКТНО с injection CB safety net. Circuit breaker pattern КОРРЕКТЕН с исправлениями.
