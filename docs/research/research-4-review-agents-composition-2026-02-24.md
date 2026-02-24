# Research 4: Состав Review-агентов

**Дата:** 2026-02-24
**Вопрос:** Какой набор параллельных sub-агентов использовать для review в Ralph loop?

---

## 1. Эталонные наборы

### 1.1 ralphex (5 агентов — дефолт)

| Агент | Фаза | Что проверяет |
|-------|-------|--------------|
| **quality** | 1st + final | Баги, security, race conditions, error handling |
| **implementation** | 1st + final | Код реально решает поставленную задачу |
| **testing** | 1st | Покрытие тестами, edge cases, качество тестов |
| **simplification** | 1st | Over-engineering, лишняя сложность |
| **documentation** | 1st | README, комментарии, docs актуальны |

**Конфигурация:** файлы `.txt` в `~/.config/ralphex/agents/`, YAML frontmatter для модели (`haiku`/`sonnet`/`opus`).

**Pipeline:** Phase 2 (all 5 parallel) → Phase 3 (codex external) → Phase 4 (quality + implementation only)

### 1.2 HAMY (9 агентов)

| # | Агент | Что проверяет |
|---|-------|--------------|
| 1 | **Test Runner** | Запускает тесты, репортит pass/fail |
| 2 | **Linter & Static Analysis** | Lint, type errors, IDE diagnostics |
| 3 | **Code Reviewer** | Top 5 improvements по impact/effort |
| 4 | **Security Reviewer** | Injection, auth, secrets, error leaks |
| 5 | **Quality & Style** | Complexity, dead code, duplication, conventions |
| 6 | **Test Quality** | Coverage ROI, behavior vs implementation testing |
| 7 | **Performance** | N+1 queries, blocking ops, memory leaks, hot paths |
| 8 | **Dependency & Deployment Safety** | New deps, breaking changes, rollback safety |
| 9 | **Simplification & Maintainability** | Проще можно? Atomicity? |

**Результат:** ~75% полезных находок (было <50% без sub-agents).

### 1.3 VoltAgent awesome-claude-code-subagents (14 агентов)

| Агент | Описание |
|-------|----------|
| code-reviewer | Code quality guardian |
| architect-reviewer | Architecture review |
| security-auditor | Vulnerability expert |
| performance-engineer | Performance optimization |
| test-automator | Test automation framework |
| accessibility-tester | A11y compliance |
| compliance-auditor | Regulatory compliance |
| debugger | Advanced debugging |
| error-detective | Error analysis |
| penetration-tester | Ethical hacking |
| qa-expert | Test automation |
| chaos-engineer | System resilience |
| ad-security-reviewer | Active Directory security |
| powershell-security-hardening | PowerShell hardening |

### 1.4 Nick Tune (Stop hook — 1 агент)

Один ревьюер с "критическим мышлением", фокус на:
- Naming quality (против "helper", "utils")
- Domain logic leaks
- Default fallback values

## 2. Анализ: сколько агентов нужно?

### Diminishing returns

| Количество | Покрытие | Стоимость (ходы) | Полезность |
|------------|----------|------------------|------------|
| 1 | Базовое | Минимальная | ~40-50% |
| 3 | Хорошее | Умеренная | ~65-70% |
| 5 | Полное | Средняя | ~75% |
| 9 | Исчерпывающее | Высокая | ~80% |

**Вывод:** 3-5 агентов — оптимум для баланса качество/стоимость.

### Что опустить без потери качества

| Агент | Нужен для bmad-ralph? | Почему |
|-------|:---:|--------|
| Test Runner | **Нет** — loop.sh уже запускает тесты | Дублирование |
| Linter | **Нет** — линтер запускается в loop.sh | Дублирование |
| Documentation | **Опционально** | Не критично для MVP |
| Deployment Safety | **Нет** | Нет CI/CD на MVP |
| Compliance | **Нет** | Не fintech/healthcare |
| Accessibility | **Нет** | Если нет UI или post-MVP |

## 3. Рекомендация для bmad-ralph

### MVP: 4 агента

| Агент | Модель | Фокус |
|-------|--------|-------|
| **quality** | sonnet | Баги, security, error handling, race conditions |
| **implementation** | sonnet | Код решает задачу из story, AC выполнены, нет лишнего |
| **simplification** | haiku | Over-engineering, мёртвый код, дублирование |
| **test-coverage** | sonnet | Каждый AC покрыт тестом, нет "зелёного нуля" |

**Почему именно эти:**
- **quality** — ловит реальные баги, самый ценный
- **implementation** — верифицирует что задача РЕШЕНА (не "почти")
- **simplification** — против раздувания, можно на haiku (дешевле)
- **test-coverage** — гарантирует что ATDD-lite не нарушен: тесты существуют для каждого AC, нет ситуации "0 тестов = зелёный прогон" (добавлен по рекомендации Murat, Party Mode)

**Не включены в MVP:**
- Performance → post-MVP, когда есть что оптимизировать
- Security deep → отдельный аудит перед релизом

### Production: 5 агентов

Добавить:
- **testing** (sonnet) — качество тестов, edge cases, flaky tests
- **security** (sonnet) — OWASP top 10, injection, secrets

### Формат конфигурации (совместим с ralphex)

```
# agents/quality.txt
---
model: sonnet
---
Review the code changes for bugs, security issues, race conditions,
and error handling problems. Report issues with severity
(Critical/High/Medium/Low) and specific file:line references.
Focus on non-obvious problems that tests and linters cannot catch.
```

```
# agents/implementation.txt
---
model: sonnet
---
Verify that the code changes actually achieve the task goals.
Read the original task from sprint-tasks.md and verify:
1. All acceptance criteria are met
2. No scope creep (unnecessary extras)
3. Edge cases from AC are handled
4. Integration with existing code is correct
```

```
# agents/simplification.txt
---
model: haiku
---
Check if the code could be simpler. Look for:
- Premature abstractions (used only once)
- Dead code or unused imports
- Over-engineered patterns for simple problems
- Code that could use existing utilities instead
Keep feedback concise: max 3 suggestions.
```

## Источники

- [ralphex docs — review agents](https://ralphex.com/docs/)
- [HAMY — 9 parallel agents](https://hamy.xyz/blog/2026-02_code-reviews-claude-subagents)
- [VoltAgent — awesome-claude-code-subagents](https://github.com/VoltAgent/awesome-claude-code-subagents)
- [Claude Code subagents docs](https://code.claude.com/docs/en/sub-agents)
- [Nick Tune — auto-review](https://www.oreilly.com/radar/auto-reviewing-claudes-code/)
