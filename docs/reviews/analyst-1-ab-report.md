# Analyst-1 Report: Variant A vs B — Knowledge Storage Location

**Analyst:** analyst-1 (knowledge-arch-research team)
**Date:** 2026-03-02
**Scope:** Compare `.claude/rules/ralph-*.md` (A) vs `.ralph/rules/ralph-*.md` (B) for distilled knowledge
**Confidence:** 88% (high evidence density, multiple confirming sources)
**Method:** /deep-research — web search (6 queries), 14 sources, internal code analysis

---

## 1. Executive Summary

**Recommendation: Вариант B (.ralph/rules/ + Go injection)** — с высокой уверенностью.

Вариант A привлекателен "zero code", но создаёт 3 критических проблемы:
1. **Двойная инъекция** — Epic 6 v5 подставляет ralph-*.md через `__RALPH_KNOWLEDGE__` И Claude Code загружает их авто из `.claude/rules/` = 2x токенов
2. **Bug #16299** — paths: frontmatter сломан, все файлы грузятся глобально, scope filtering невозможен
3. **Нет JIT validation** — auto-loaded контент не проходит через Go stale filter

Вариант B даёт полный контроль при ~30-40 строках Go кода. DistillState уже в .ralph/ — прецедент существует.

---

## 2. Критическая Находка: Двойная Инъекция (Bug in Epic 6 v5)

### 2.1. Проблема

Epic 6 v5 Story 6.2 использует ОБА канала одновременно:

- **Канал 1 (auto-load):** Claude Code загружает ВСЕ `.md` из `.claude/rules/` в контекст [Claude Code Docs: Memory]
- **Канал 2 (Go injection):** `buildKnowledgeReplacements()` читает ralph-*.md и подставляет через `__RALPH_KNOWLEDGE__` в Stage 2 [Epic 6 v5, Story 6.2 AC]

**Цитата из Story 6.2:** "ALL ralph-*.md files loaded from .claude/rules/ (glob pattern)" + "combined content injected via strings.Replace __RALPH_KNOWLEDGE__"

**Результат:** ralph-testing.md, ralph-errors.md и другие будут в контексте ДВАЖДЫ.

### 2.2. Масштаб

| Метрика | Значение |
|---------|----------|
| Ожидаемые ralph-*.md файлов | 7-8 категорий + index + misc + critical |
| Токенов на файл (est.) | ~500-1500 |
| Двойной overhead | ~7K-12K токенов |
| % от 200K контекста | 3.5-6% |
| Существующие rules (проект) | 9 файлов, 122 паттерна (~15K tokens) |

### 2.3. Подтверждающие источники

- **P2-M7** [pair2-architecture-review.md:159]: "Entries прошедшие distillation присутствуют в ОБОИХ источниках" — partial duplication identified
- **J10** [review-6.5-6.6-consensus.md:142]: "ralph-misc.md loaded only через ralph prompt injection, не через Claude Code auto-load" — implicit acknowledgment
- **Bug #16299** [GitHub]: paths: frontmatter broken → ALL .md files load globally → no way to prevent auto-load
- **Context Optimization** [GitHub Gist]: 54% token reduction possible by eliminating duplication

---

## 3. Вариант A: ralph-*.md в .claude/rules/ (auto-load)

### Плюсы

| # | Плюс | Детали |
|---|------|--------|
| A1 | **Zero injection code** | Нет Go кода для чтения/injection (~30-40 строк saved) |
| A2 | **Proven infrastructure** | 9 файлов/122 паттерна уже работают в .claude/rules/ |
| A3 | **System prompt priority** | Контент из .claude/rules/ = system prompt level, наравне с CLAUDE.md [Claude Code docs] |
| A4 | **Survives compaction** | Re-injected fresh после /compact [Claude Code docs] |
| A5 | **Ecosystem совместимость** | Subagents, skills, hooks видят файлы из .claude/rules/ |

### Минусы

| # | Минус | Severity | Детали |
|---|-------|----------|--------|
| A6 | **Bug #16299: scope broken** | CRITICAL | paths: frontmatter ignored → все файлы грузятся ВСЕГДА. Открыт Jan 2026, не исправлен [GitHub #16299] |
| A7 | **Bug #17204: YAML syntax broken** | HIGH | Документированный формат paths: не работает. Только недокументированный globs: [GitHub #17204] |
| A8 | **Двойная инъекция** | HIGH | Auto-load + Go injection = 2x tokens (см. раздел 2) |
| A9 | **Namespace mixing** | MEDIUM | 9 user rules + 10 ralph files = 19 файлов в одной директории |
| A10 | **Нет JIT validation** | HIGH | Go не участвует в auto-load → не может фильтровать stale entries (ValidateLearnings невозможен) |
| A11 | **Порядок неконтролируем** | MEDIUM | Claude Code определяет порядок. Reverse read (L3 newest-first) невозможен |
| A12 | **CVE risk** | MEDIUM | CVE-2025-59536, CVE-2026-21852: .claude/ = confirmed attack surface. Программатическая запись увеличивает risk [Check Point Research] |
| A13 | **Pipe mode uncertainty** | MEDIUM | -p mode loading .claude/rules/ не гарантирован в docs [Claude Code CLI ref] |

---

## 4. Вариант B: ralph-*.md в .ralph/rules/ (Go injection)

### Плюсы

| # | Плюс | Детали |
|---|------|--------|
| B1 | **Полный injection контроль** | Go решает КАКИЕ файлы, В КАКОМ порядке, СКОЛЬКО контента |
| B2 | **Go scope filtering** | Own implementation, не зависит от сломанного paths: |
| B3 | **Namespace isolation** | .ralph/rules/ изолирован от user .claude/rules/ |
| B4 | **Budget control** | Go ограничивает суммарный injection до learnings_budget |
| B5 | **Stage 2 safety** | strings.Replace в Stage 2 = proven, safe from template injection [config/prompt.go] |
| B6 | **Независимость** | Не зависит от Claude Code auto-load behavior/bugs/changes |
| B7 | **Тестируемость** | Unit tests + golden files для injection logic |
| B8 | **Prompt positioning** | Ralph определяет позицию в primacy zone |
| B9 | **Нет двойной загрузки** | Единственный канал = zero duplication |
| B10 | **CVE isolation** | .ralph/ ≠ .claude/ → не в attack surface CVE-2025-59536 |
| B11 | **Прецедент** | DistillState уже в .ralph/distill-state.json [Epic 6 v5] |
| B12 | **JIT validation** | ValidateLearnings работает как designed — Go фильтрует stale перед injection |

### Минусы

| # | Минус | Severity | Детали |
|---|-------|----------|--------|
| B13 | **Доп. Go код** | LOW | ~30-40 строк: filepath.Glob + os.ReadFile + strings.Replace |
| B14 | **Нет auto-load** | LOW | Файлы не видны при ручном `claude` (не через ralph). Mitigation: ralph знания для ralph сессий |
| B15 | **Ещё одна директория** | LOW | .ralph/ уже существует (distill-state.json) |
| B16 | **User prompt position** | LOW | Может иметь чуть меньший instruction weight чем system prompt |

---

## 5. Сравнительная Матрица

| Критерий | A (.claude/rules/) | B (.ralph/rules/) | Победитель |
|----------|--------------------|--------------------|------------|
| Код complexity | 0 строк | ~30-40 строк Go | A (minor) |
| Scope filtering | Сломан (#16299, #17204) | Go реализация | **B** |
| Двойная инъекция | **ДА (bug in v5)** | Нет | **B** |
| JIT stale filtering | Невозможно | Полный | **B** |
| Recency ordering (L3) | Невозможно | Детерминистический | **B** |
| Namespace isolation | Mixed (19 files) | Isolated | **B** |
| Pipe mode reliability | Uncertain | Explicit injection | **B** |
| Тестируемость | Black box | Full (unit+golden) | **B** |
| CVE risk | Increased | Isolated | **B** |
| Budget control | All-or-nothing | Precise | **B** |
| Ecosystem compat | Subagents see files | Only ralph sessions | A |
| Compaction survival | Automatic | Per-session injection | Tie |
| **Итого** | **2 wins** | **10 wins** | **B** |

---

## 6. Анализ: Возможен ли Гибрид A+B?

### 6.1. Гибрид "A+B Fixed" (distilled auto-load + raw injection)

Вариант: ralph-{category}.md в `.claude/rules/` (auto-load), LEARNINGS.md через Go injection, `__RALPH_KNOWLEDGE__` НЕ дублирует auto-loaded content.

**Проблемы:**
- Всё ещё зависит от Bug #16299 (no scope control)
- Всё ещё namespace mixing
- Требует УДАЛЕНИЯ `__RALPH_KNOWLEDGE__` placeholder или перепрофилирования его в metadata-only (~50 tokens)
- JIT validation для ralph-*.md невозможен (auto-loaded)

**Оценка:** Лучше чем чистый A, но хуже чем B. Unnecessary complexity без выигрыша.

### 6.2. Гибрид "B + symlink" (inject + expose to ecosystem)

Вариант: файлы в `.ralph/rules/`, symlink из `.claude/rules/ralph-*.md` → `.ralph/rules/ralph-*.md`

**Проблемы:**
- Symlinks на WSL/NTFS ненадёжны [wsl-ntfs.md patterns]
- Двойная загрузка через symlink = та же проблема что вариант A
- Дополнительная complexity без пользы

**Оценка:** Не рекомендуется.

---

## 7. Рекомендация

### Вариант B — .ralph/rules/ + Go injection через __RALPH_KNOWLEDGE__

**Confidence: 88%**

#### Обоснование

1. **Bug #16299 = deal-breaker для A.** Scope filtering сломан. Нет ETA на fix. Все файлы грузятся всегда. Для ralph-знаний "всё грузится" может быть приемлемо, но это всё равно двойная инъекция если Go тоже подставляет content.

2. **Двойная инъекция = wasted tokens.** ~7-12K токенов дублирования при каждой сессии. При 200K контексте это 3.5-6% — значимо. "Smallest set of high-signal tokens" [Anthropic Context Engineering].

3. **JIT validation = architectural requirement.** ValidateLearnings + stale filtering через os.Stat — core feature Epic 6. Невозможен без Go participation в injection pipeline.

4. **DistillState прецедент.** .ralph/ уже принят для state. Rules — логическое расширение.

5. **Minimal code cost.** 30-40 строк Go vs 10 architectural risks варианта A — тривиальный tradeoff.

6. **CVE defense.** Минимизация записи в .claude/ = security best practice.

#### Изменения к Epic 6 v5

| Story | Изменение | Причина |
|-------|-----------|---------|
| 6.5b | Output destination: `.ralph/rules/ralph-{category}.md` вместо `.claude/rules/` | Prevent double injection |
| 6.2 | `__RALPH_KNOWLEDGE__` = Go reads from `.ralph/rules/` | Full control |
| 6.2 | `buildKnowledgeReplacements` reads from `.ralph/rules/` | Consistent path |
| 6.5b | ralph-index.md → `.ralph/rules/` | Co-located with data |
| 6.5c | Backup scope includes `.ralph/rules/ralph-*.md` | Already in .ralph/ |
| 6.6 | `ralph distill` output → `.ralph/rules/` | Consistent |

#### Risk: Subagent/Skill Access

Ralph-generated knowledge в `.ralph/rules/` не видно Claude Code subagents/skills напрямую. **Mitigation:** Ralph всегда подаёт knowledge через prompt injection — это его архитектурная задача. Subagents в ralph запускаются с assembled prompt, который уже содержит knowledge.

---

## 8. Evidence Table

| ID | Source | Type | Quality | Key Finding |
|----|--------|------|---------|-------------|
| S1 | [Claude Code Memory docs](https://code.claude.com/docs/en/memory) | Official | A | All .md in .claude/rules/ auto-loaded in system prompt |
| S2 | [GitHub #16299](https://github.com/anthropics/claude-code/issues/16299) | Bug | A | paths: frontmatter broken, all files load globally. Open since Jan 2026 |
| S3 | [GitHub #17204](https://github.com/anthropics/claude-code/issues/17204) | Bug | A | Documented paths: YAML syntax broken. Only undocumented globs: works |
| S4 | [GitHub #13905](https://github.com/anthropics/claude-code/issues/13905) | Bug | A | Invalid YAML syntax in rules frontmatter |
| S5 | [ClaudeFast: Rules Directory](https://claudefa.st/blog/guide/mechanics/rules-directory) | Guide | B | Rules injected into system prompt, modular |
| S6 | [SFEIR: CLAUDE.md](https://institute.sfeir.com/en/claude-code/claude-code-memory-system-claude-md/faq/) | Research | B | ~15 rules = 94% compliance, degradation beyond |
| S7 | [Context Optimization Gist](https://gist.github.com/johnlindquist/849b813e76039a908d962b2f0923dc9a) | Community | B | 54% token reduction by eliminating system prompt duplication |
| S8 | [Anthropic Context Engineering](https://01.me/en/2025/12/context-engineering-from-claude/) | Analysis | B | "smallest set of high-signal tokens" |
| S9 | [GitHub #28984](https://github.com/anthropics/claude-code/issues/28984) | Feature req | B | Community requests to reduce compaction overhead |
| S10 | [Check Point CVE-2025-59536](https://research.checkpoint.com/2026/) | Security | A | .claude/ project files = confirmed attack surface |
| S11 | pair2-architecture-review.md:159 | Internal | A | P2-M7: partial duplication between LEARNINGS and ralph-*.md |
| S12 | pair3-analyst-injection-review.md:233 | Internal | A | Stage 2 injection confirmed safe |
| S13 | review-6.5-6.6-consensus.md:142 | Internal | A | J10: ralph-misc.md no auto-load, injection only |
| S14 | Epic 6 v5 spec | Internal | A | Current design: dual-channel injection (the bug) |

---

## 9. Conclusion

Вариант B предпочтителен по 10 из 12 критериев. Единственные преимущества варианта A — zero code (minor) и ecosystem compatibility (mitigation exists). Критические недостатки A — двойная инъекция, сломанный scope filtering, невозможность JIT validation — не имеют workaround.

Стоимость варианта B — ~30-40 строк Go кода — тривиальна по сравнению с архитектурными рисками варианта A.

**Bottom line:** Файлы в `.ralph/rules/`, injection через `__RALPH_KNOWLEDGE__`, Go контролирует весь pipeline. DistillState прецедент подтверждает жизнеспособность .ralph/ как storage location.
