# Автоматическая генерация scope-aware rules через LLM дистилляцию — Исследование

**Дата:** 2026-03-02
**Контекст:** Валидация подхода bmad-ralph Epic 6 — автоматическое создание ralph-{category}.md с YAML frontmatter и glob-паттернами через LLM дистилляцию
**Аудитория:** Технический лид / Scrum Master проекта bmad-ralph

---

## Executive Summary

- Подход bmad-ralph (LLM автоматически генерирует scope-aware rules файлы с glob-паттернами) — **пионерский**. Ни один существующий инструмент не делает это полностью автоматически
- Индустрия движется в этом направлении: Cursor, Claude Code, Copilot уже поддерживают glob-scoped rules, но **все требуют ручного создания**
- OpenAI Harness Engineering подтверждает ключевые принципы: knowledge-as-code, mechanical enforcement, doc-gardening agents
- Copilot Memory (декабрь 2025) — первый шаг конкурентов к автоматическому обучению, но без scope-aware файлов
- **Риски подхода реальны** но управляемы: невалидные globs, category drift, model collapse — все покрываются Go-валидацией (уже в дизайне)

---

## Методология

Изучены 15+ источников:
- OpenAI Harness Engineering (оригинал + InfoQ + Martin Fowler анализ)
- Cursor Rules (официальная документация)
- Claude Code rules и skills (документация + best practices)
- GitHub Copilot instructions + Memory (декабрь 2025 - февраль 2026)
- Aider conventions
- Claude-Mem progressive disclosure
- Google ADK framework (декабрь 2025)
- Microsoft Azure SRE Agent (январь 2026)
- Академические работы: "Codified Context" (Peking University, 2026), Structured Agent Distillation (2025)
- Anthropic "Effective Context Engineering" (2025)

---

## Ключевые находки

### 1. Все основные инструменты поддерживают glob-scoped rules — но все вручную

| Инструмент | Формат rules | Glob scoping | Автоматическая генерация |
|------------|-------------|-------------|------------------------|
| **Cursor** | `.cursor/rules/*.mdc` | Да, YAML frontmatter `globs:` | Нет — ручное создание |
| **Claude Code** | `.claude/rules/*.md` | Да, YAML frontmatter `globs:` | Нет — ручное создание |
| **GitHub Copilot** | `.github/instructions/*.md` | Да, file patterns | Нет — ручное (но Memory учится) |
| **OpenAI Codex** | `AGENTS.md` + `docs/` | Нет globs — единый файл | Нет (doc-gardening agents обновляют) |
| **Aider** | `CONVENTIONS.md` | Нет — единый файл | Нет — ручное |
| **bmad-ralph** | `.claude/rules/ralph-*.md` | Да, YAML `globs:` | **Да — LLM дистилляция** |

**Вывод:** bmad-ralph — единственный инструмент, который планирует **автоматически генерировать** scope-aware rules файлы. Все конкуренты полагаются на ручное создание.

[Cursor Rules Docs](https://cursor.com/docs/context/rules) | [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices)

---

### 2. OpenAI Harness Engineering — подтверждение ключевых принципов

OpenAI за 5 месяцев создал ~1 млн строк кода с командой из 3 инженеров, используя Codex агентов. Их подход к knowledge management:

**Что совпадает с bmad-ralph:**
- **Knowledge-as-code:** "Если знания не в репозитории — для агента их не существует" [OpenAI Harness Engineering](https://openai.com/index/harness-engineering/)
- **Mechanical enforcement:** Линтеры и CI валидируют что knowledge base актуален и структурирован [InfoQ, Feb 2026](https://www.infoq.com/news/2026/02/openai-harness-engineering-codex/)
- **Doc-gardening agents:** Агенты периодически сканируют на stale/obsolete документацию и открывают fix-up PR — аналог авто-дистилляции bmad-ralph [Martin Fowler](https://martinfowler.com/articles/exploring-gen-ai/harness-engineering.html)
- **AGENTS.md как table of contents:** ~100 строк, указатели на детальные docs/ — аналог ralph-index.md
- **Depth-first:** "Когда агент не справляется — диагностируй что не хватает и пусть агент сам это построит"

**Чего нет у OpenAI (есть у bmad-ralph):**
- **Glob-scoped rules:** AGENTS.md — единый файл без scoping
- **Автоматическая дистилляция:** Doc-gardening agents обновляют, но не сжимают/реорганизуют
- **Quality gates на запись:** Нет программных фильтров на контент knowledge base
- **Circuit breaker:** Нет protection от failure каскадов

---

### 3. Copilot Memory — первый шаг конкурентов к автоматике

GitHub Copilot запустил Memory в декабре 2025 (Pro/Pro+):

- **Агенты учатся из кодовой базы** — автоматически захватывают инсайты [GitHub Blog, Dec 2025](https://github.blog/changelog/2025-12-19-copilot-memory-early-access-for-pro-and-pro/)
- **Cross-agent memory:** coding agent, CLI, code review делят общую память
- **Но:** Нет scope-aware файлов, нет glob-паттернов, нет distillation pipeline
- **Но:** Closed-source, нет контроля над форматом и содержимым памяти

**Вывод:** Copilot движется в сторону автоматического обучения, но без scope-aware rules и без прозрачности. bmad-ralph даёт полный контроль через version-controlled файлы.

---

### 4. Progressive Disclosure — индустриальный консенсус

Все крупные игроки конвергируют к tiered/progressive disclosure:

| Источник | Реализация |
|----------|-----------|
| **Anthropic** | "Agents navigate and retrieve data autonomously — progressive disclosure allows agents to discover relevant context through exploration" [Anthropic Context Engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) |
| **Claude-Mem** | 3 слоя: Index (metadata) → Timeline (context) → Details (full content) [Claude-Mem Docs](https://docs.claude-mem.ai/progressive-disclosure) |
| **Google ADK** | "Scope by default: every model call sees minimum context required" [Google Developers Blog, Dec 2025](https://developers.googleblog.com/architecting-efficient-context-aware-multi-agent-framework-for-production/) |
| **Microsoft** | "Moved domain knowledge from system prompts into files agents read on-demand" [MS Tech Community, Jan 2026](https://techcommunity.microsoft.com/blog/appsonazureblog/context-engineering-lessons-from-building-azure-sre-agent/4481200/) |
| **Peking University** | "Hot-memory constitution + cold-memory knowledge base of 34 on-demand specs" [arXiv, 2026](https://arxiv.org/html/2602.20478) |
| **Agent Skills standard** | "Progressive disclosure: agents load only skill names initially; full content on-demand" [Codex Skills](https://developers.openai.com/codex/skills/) |

**bmad-ralph T1/T2/T3 полностью соответствует этому консенсусу:**
- T1 (ralph-critical.md, always loaded) = hot memory
- T2 (ralph-{category}.md, glob-scoped) = scoped on-demand
- T3 (LEARNINGS.md, full injection) = cold storage

---

### 5. Автоматическая генерация rules — нет прямых аналогов

**Никто в индустрии не делает то что планирует bmad-ralph:** автоматическая генерация glob-scoped rules файлов через LLM дистилляцию.

Ближайшие аналоги:
- **OpenAI doc-gardening agents** — обновляют документацию, но не генерируют scoped rules
- **Copilot Memory** — автоматически учится, но не создаёт файлы
- **Claude-Mem** — structured extraction, но ручная организация
- **Qodo** — учится из PR history, но не создаёт rules файлы

**Это означает:**
1. bmad-ralph — пионер в этом подходе
2. Нет доказанных best practices — придётся создавать свои
3. Go-валидация (YAML, globs, content) — критически важна как safety net
4. Нужен fallback при неудаче (backup + GATE на человека — уже в дизайне)

---

### 6. Риски автоматической генерации и их митигация

| Риск | Источник | Митигация в bmad-ralph |
|------|----------|----------------------|
| **Невалидные globs** | Claude может написать `*.test.go` вместо `*_test.go` — rules не загрузятся | Go `filepath.Match` валидация + проверка что хотя бы 1 файл матчится (M4/M8) |
| **Category drift** | "testing" / "tests" / "test-patterns" — множатся файлы | Canonical category list + NEW_CATEGORY протокол (H2) |
| **Model collapse** | Повторная дистилляция теряет edge cases [Nature 2024] | "Last 20% preserved" валидация + backup (H5) |
| **Hallucinated rules** | LLM может добавить правила которых не было в исходных данных [LLM Scaling Paradox, Feb 2026] | ValidateDistillation: "no new citations not in input" (Пара 2 рекомендация) |
| **Stale globs** | Проект эволюционирует, расширения меняются | Go пересканирует file types при каждой дистилляции (M4) |
| **Oversized rules** | Один ralph-*.md > 500 строк — перегрузка контекста | Cursor best practice: <500 строк per rule file. Можно добавить Go проверку |
| **Broken YAML** | Невалидный frontmatter → файл не загружается | yaml.Unmarshal валидация (M8) |

**Вывод:** Все риски уже покрыты в дизайне Epic 6 через Go-валидацию. Подход рискованный но управляемый.

---

### 7. Cursor .mdc формат — эталон для file-scoped rules

Cursor наиболее развит в области scoped rules. Их формат:

```yaml
---
description: "Правила для React компонентов"
alwaysApply: false
globs: ["**/*.tsx", "**/*.jsx"]
---

# React Component Rules
- Используй functional components
- Props через interface, не type
```

**Четыре типа применения:**
1. **Always Apply** (`alwaysApply: true`) — для каждой сессии
2. **Auto Attached** — файл подключается когда globs матчит
3. **Agent Decision** — агент решает по description
4. **Manual** — пользователь вызывает `@ruleName`

Claude Code `.claude/rules/*.md` использует похожий формат (YAML frontmatter с `globs:`), но без `description` и `alwaysApply` полей.

**bmad-ralph ralph-*.md файлы = Auto Attached тип Cursor**, что является самым востребованным типом для project-specific knowledge.

[Cursor Rules Documentation](https://cursor.com/docs/context/rules)

---

## Анализ

### Тема 1: bmad-ralph как first-mover в автоматической генерации

Индустрия разделилась на два лагеря:
1. **Manual curation** (Cursor, Claude Code, Aider, Codex) — разработчик вручную пишет rules
2. **Automatic learning** (Copilot Memory, Qodo) — система учится, но не создаёт файлов

bmad-ralph занимает **уникальную нишу**: автоматическое обучение + автоматическая генерация version-controlled rules файлов. Это объединяет прозрачность manual подхода с масштабируемостью automatic learning.

OpenAI Harness Engineering подтверждает жизнеспособность: их doc-gardening agents уже **обновляют** knowledge base автоматически. bmad-ralph идёт на шаг дальше — не просто обновляет, а **создаёт и организует** по scope.

### Тема 2: Go-валидация как критический safety net

Ни один конкурент не имеет программных quality gates на содержимое rules файлов. Это не случайность — при ручном создании человек является quality gate.

При **автоматической** генерации quality gates обязательны. bmad-ralph дизайн включает:
- YAML frontmatter валидация
- Glob syntax + match проверка
- Content preservation (last 20%)
- No-hallucination check (no new citations)
- Category consistency

Это аналогично тому как OpenAI использует "structural tests validating compliance" и "dedicated linters", но применительно к knowledge файлам а не к коду.

### Тема 3: Progressive disclosure — доказанный паттерн

T1/T2/T3 архитектура bmad-ralph полностью совпадает с индустриальным консенсусом 2025-2026:
- Anthropic: "progressive disclosure allows agents to discover relevant context through exploration"
- Google: "scope by default — every model call sees minimum context required"
- Microsoft: "moved domain knowledge into files agents read on-demand"
- Claude-Mem: "two-tier everything: index first, details on-demand"
- Peking University: "hot-memory constitution + cold-memory knowledge base"

Glob-based scoping (используемый Cursor и Claude Code) — зрелый механизм с широким adoption.

---

## Риски и ограничения исследования

1. **Нет прямых аналогов** для сравнения — bmad-ralph первый в этой нише. Нельзя сказать "X делает так же и работает"
2. **Copilot Memory** — closed-source, невозможно сравнить техническую реализацию
3. **OpenAI Harness Engineering** — описан на высоком уровне, без деталей реализации doc-gardening agents
4. **Cursor rules** — хорошо документирован формат, но нет данных об automated rule management
5. **Исследование ограничено публичными источниками** — внутренние практики крупных команд недоступны

---

## Рекомендации

### Подтверждено исследованием — делать как запланировано:

1. **Glob-scoped ralph-*.md файлы** — индустриальный стандарт (Cursor, Claude Code)
2. **Progressive disclosure T1/T2/T3** — консенсус 6+ крупных источников
3. **Go-валидация YAML + globs + content** — обязательна при автоматической генерации (нет аналогов для заимствования best practices)
4. **Knowledge-as-code (version-controlled)** — подтверждён OpenAI Harness Engineering
5. **Backup перед дистилляцией** — safety net, критично для пионерского подхода

### Дополнительные рекомендации из исследования:

6. **Размер одного rule файла < 500 строк** — Cursor best practice, имеет смысл добавить как Go проверку
7. **Description поле в frontmatter** — Cursor использует для agent-driven discovery. Рассмотреть для ralph-*.md (Growth phase)
8. **Метрики эффективности** — OpenAI отслеживает throughput (3.5 PR/инженер/день). bmad-ralph может отслеживать: review findings до/после knowledge injection
9. **Fallback ralph-misc.md** — для записей без подходящего scope. Но НЕ давать `globs: ["**"]` (иначе загружается всегда — монолит)

### Чего НЕ делать:

10. **Не добавлять embeddings/RAG** — file-based injection достаточен для <500 entries (R3 research)
11. **Не копировать Copilot Memory** — closed-source approach без прозрачности
12. **Не усложнять формат** — Cursor и Claude Code используют простой YAML + markdown, не JSON/TOML

---

## Appendix A: Evidence Table

| Claim | Source | Quality |
|-------|--------|---------|
| Glob-scoped rules — индустриальный стандарт | Cursor Docs, Claude Code Docs | A |
| OpenAI: knowledge-as-code, doc-gardening agents | OpenAI Blog, InfoQ, Martin Fowler | A |
| Progressive disclosure — консенсус | Anthropic, Google, Microsoft, Claude-Mem, Peking U | A |
| Copilot Memory — автоматическое обучение без rule files | GitHub Blog, DEV Community | B |
| Rules < 500 строк best practice | Cursor Docs, WorkOS Blog | B |
| LLM может генерировать невалидные globs | Inference from LLM limitations research | B |
| Model collapse при повторной дистилляции | Nature 2024, LLM Scaling Paradox Feb 2026 | A |
| Никто не делает автоматическую генерацию scoped rules | Отсутствие результатов в 3 целевых поисках | B |

## Appendix B: Sources

- [OpenAI Harness Engineering](https://openai.com/index/harness-engineering/)
- [OpenAI Harness Engineering — InfoQ Analysis, Feb 2026](https://www.infoq.com/news/2026/02/openai-harness-engineering-codex/)
- [Martin Fowler — Harness Engineering Analysis](https://martinfowler.com/articles/exploring-gen-ai/harness-engineering.html)
- [Cursor Rules Documentation](https://cursor.com/docs/context/rules)
- [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices)
- [GitHub Copilot Memory — Early Access, Dec 2025](https://github.blog/changelog/2025-12-19-copilot-memory-early-access-for-pro-and-pro/)
- [GitHub Copilot Custom Instructions Guide, Feb 2026](https://smartscope.blog/en/generative-ai/github-copilot/github-copilot-custom-instructions-guide/)
- [Codex Agent Skills](https://developers.openai.com/codex/skills/)
- [Aider Conventions](https://aider.chat/docs/usage/conventions.html)
- [Anthropic — Effective Context Engineering for AI Agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
- [Claude-Mem Progressive Disclosure](https://docs.claude-mem.ai/progressive-disclosure)
- [Google ADK — Context-Aware Multi-Agent Framework, Dec 2025](https://developers.googleblog.com/architecting-efficient-context-aware-multi-agent-framework-for-production/)
- [Microsoft Azure SRE Agent — Context Engineering, Jan 2026](https://techcommunity.microsoft.com/blog/appsonazureblog/context-engineering-lessons-from-building-azure-sre-agent/4481200/)
- [Codified Context — Peking University, 2026](https://arxiv.org/html/2602.20478)
- [Anthropic 2026 Agentic Coding Trends Report](https://resources.anthropic.com/hubfs/2026%20Agentic%20Coding%20Trends%20Report.pdf)
- [AI Agentic Programming: A Survey — arXiv, 2025](https://arxiv.org/html/2508.11126v1)
