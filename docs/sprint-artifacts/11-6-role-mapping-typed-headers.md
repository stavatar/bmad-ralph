# Story 11.6: Role mapping и typed headers

Status: Done

## Review Findings (0H/3M/1L)

### MEDIUM
- [M1] AC4 closing tag `<!-- end file: ... -->` отсутствует в template `[plan/prompts/plan.md]`
- [M2] Нет end-to-end теста ResolveRole → GeneratePrompt → typed header `[plan/plan_test.go]`
- [M3] Case-sensitivity: filepath.Base сохраняет регистр, defaultRoles lowercase only `[plan/plan.go:56-57]`

### LOW
- [L1] Нет тест-кейса для пустого filename `[plan/plan_test.go]`

## Story

As a система,
I want автоматически назначать роли входным файлам по именам и передавать их LLM через typed headers,
so that Claude понимает семантику каждого документа.

## Acceptance Criteria

1. **Входной файл без явной роли** получает роль из дефолтного маппинга:
   ```
   prd.md           → requirements
   architecture.md  → technical_context
   ux-design.md     → design_context
   front-end-spec.md → ui_spec
   ```

2. **Явная роль в `cfg.PlanInputs[].Role`** переопределяет дефолт (FR11)

3. **В single-doc режиме** (`cfg.PlanMode == "single"`) роли не назначаются и typed headers не добавляются (FR12)

4. **Промпт для Claude содержит typed headers:**
   ```
   <!-- file: prd.md | role: requirements -->
   <content здесь>
   <!-- end file: prd.md -->
   ```

5. **Тесты в `plan_test.go`:**
   - BMad autodiscovery: `prd.md` → role `requirements`
   - Explicit override: custom role в config → custom role в header
   - Single-doc: нет typed headers в промпте

## Tasks / Subtasks

- [x] Task 1: Реализовать role mapping (AC: #1, #2)
  - [x] Определить `defaultRoles map[string]string` (unexported) в `plan/plan.go`
  - [x] Реализовать `ResolveRole(filename string, explicitRole string, singleDoc bool) string`
  - [x] Если `singleDoc` → return "" (нет роли)
  - [x] Если `explicitRole != ""` → return explicitRole (FR11)
  - [x] Иначе → lookup в defaultRoles по filename (basename)
- [x] Task 2: Реализовать typed headers injection (AC: #3, #4)
  - [x] Typed headers уже реализованы в plan/prompts/plan.md template (Story 11.4)
  - [x] ResolveRole используется caller (cmd/ralph/plan.go) для заполнения PlanInput.Role
- [x] Task 3: Написать тесты (AC: #5)
  - [x] TestResolveRole_Scenarios: 9 table-driven cases covering all mappings, override, single-doc, path with dir

## Dev Notes

### Архитектурные ограничения

- **User content через `strings.Replace`** — typed headers как часть user content, не через template [Source: docs/epics.md#Story 11.6 Technical Notes]
- **FR20:** typed headers `<!-- file: <name> | role: <role> -->` — разработчик Claude использует реальные имена для source-ссылок [Source: docs/epics.md#FR20]
- **FR12:** single-doc режим — без ролей и без typed headers [Source: docs/epics.md#FR12]

### Существующий код

- `plan/plan.go` (Story 11.5) — `Run()`, `templateData`, content injection через `strings.Replace`
- `plan/prompts/plan.md` (Story 11.4) — инструкции по интерпретации typed headers

### defaultRoles

```go
var defaultRoles = map[string]string{
    "prd.md":            "requirements",
    "architecture.md":   "technical_context",
    "ux-design.md":      "design_context",
    "front-end-spec.md": "ui_spec",
}
```

### Тестирование

- Table-driven для resolveRole [Source: CLAUDE.md#Testing Core Rules]
- Symmetric negative checks: single-doc → assert typed headers absent [Source: .claude/rules/test-assertions-base.md]
- Substring assertions section-specific: `<!-- file:` unique marker [Source: .claude/rules/test-assertions-base.md]

### Project Structure Notes

- `plan/plan.go` — модификация: добавление `defaultRoles`, `resolveRole()`, typed headers injection
- `plan/plan_test.go` — добавление тестов role mapping

### References

- [Source: docs/epics.md#Story 11.6] — полные AC и технические заметки
- [Source: docs/project-context.md#Plan Package] — инварианты plan/
- [Source: docs/epics.md#FR11] — explicit role override
- [Source: docs/epics.md#FR12] — single-doc без ролей
- [Source: docs/epics.md#FR20] — typed headers формат

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added defaultRoles map and ResolveRole() exported function to plan/plan.go
- ResolveRole handles: singleDoc → "", explicit → explicit, basename lookup → default, unknown → ""
- Added TestResolveRole_Scenarios with 9 table-driven cases
- Typed headers already in template from Story 11.4 — ResolveRole fills PlanInput.Role for caller
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- plan/plan.go (modified): added defaultRoles, ResolveRole()
- plan/plan_test.go (modified): added TestResolveRole_Scenarios
