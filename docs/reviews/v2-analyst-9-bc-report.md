# Analyst-9: Вариант B vs C — Глубокий сравнительный анализ

**Дата:** 2026-03-02
**Аналитик:** analyst-9 (knowledge-arch-v2)
**Scope:** Go injection only (B) vs Дублирование обоими каналами (C)
**Метод:** Deep research + синтез 6 документов (2 аналитических отчёта, 3 исследования, 1 мета-анализ)

---

## Executive Summary

Вариант B (`.ralph/rules/` + Go injection) и вариант C (`.claude/rules/ralph-*.md` + Go injection одновременно) — два архитектурно жизнеспособных подхода. Ключевое различие: C добавляет второй канал доставки через auto-load Claude Code, надеясь на compliance boost от prompt repetition. Однако **при объективном взвешивании всех факторов вариант B является оптимальным**, а выгода от дублирования в варианте C — **теоретическая и невоспроизводимая** в контексте Ralph.

**Confidence:** 88% за B.

Главная причина: дублирование через `.claude/rules/` вместо чистого усиления создаёт **архитектурную зависимость от чужой экосистемы** (disclaimer framing, Bug #16299, CVE surface, untestable channel), а measured compliance gain от repetition **нерелевантен для reasoning tasks** и **не масштабируется** с ростом базы знаний.

---

## 1. Что именно сравнивается

### Вариант B: Go injection only

```
.ralph/rules/ralph-*.md          ← Ralph-owned directory
       ↓
Go читает файлы → AssemblePrompt()
       ↓
__RALPH_KNOWLEDGE__ placeholder → user prompt
       ↓
claude --print получает промпт с инлайн-знаниями
```

**Единственный канал.** Go полностью контролирует что, когда и как попадает в контекст модели.

### Вариант C: Дублирование (оба канала)

```
.claude/rules/ralph-*.md          ← Auto-loaded by Claude Code
       ↓                                    ↓
Claude Code загружает             Go читает те же файлы
как "project instructions"        → AssemblePrompt()
       ↓                                    ↓
system-reminder context    +    __RALPH_KNOWLEDGE__ → user prompt
       ↓                                    ↓
               Контент в контексте ДВАЖДЫ
```

**Два канала.** Один файл, два пути доставки. Claude Code видит ralph-*.md автоматически И Go подставляет тот же контент в промпт.

---

## 2. Анализ prompt repetition research

### 2.1 Google Research (arxiv:2512.14982) — что реально показано

**Методология:** Verbatim duplication `<QUERY>` → `<QUERY><QUERY>`. 7 моделей × 7 бенчмарков.

**Результаты:**

| Режим | Wins | Losses | Ties | Interpretation |
|-------|------|--------|------|----------------|
| Non-reasoning | **47/70** | **0/70** | 23/70 | Стабильно полезно |
| With reasoning | 5/28 | 1/28 | 22/28 | Практически нейтрально |

**Критическое различие для Ralph:**

Ralph оркестрирует Claude Code через `claude --print`. Claude с thinking mode = reasoning model. Исследование явно показывает: **reasoning модели минимально выигрывают от repetition** (5 wins из 28, т.е. 18% vs 67% для non-reasoning). Причина: reasoning модели через RL *сами учатся* повторять промпт внутренне [arxiv:2512.14982, §4.2].

Analyst-2 корректно цитирует 47/70 wins, но **не разделяет reasoning vs non-reasoning**, что создаёт завышенное впечатление о выгоде для Ralph.

### 2.2 Lost in the Middle — неприменимость к instructions

**Факт:** U-образная кривая внимания подтверждена [arxiv:2307.03172, TACL 2024].

**Но:** Analyst-1 (§6.2) со ссылкой на analyst-5 установил критическое различие:

> Lost in the Middle исследовалось на задачах **поиска факта в массе документов** (retrieval tasks). System prompt instructions — **другой жанр**: модели применяют их как правила поведения (imperative mode), а не ищут конкретный факт (retrieval mode).

Модели обучены следовать system prompt instructions **целиком**, не "искать нужную инструкцию среди массы ненужных." При <10K токенов instructions Lost in the Middle **вообще не применим**.

Ralph-знания = ~1-3K tokens. Аргумент "dual position reduces lost-in-middle risk" (analyst-2, C2) **необоснован** для данного масштаба.

### 2.3 IFScale benchmark (arxiv:2507.11538) — новые данные

Deep research выявил исследование 2025 года:

| Тип модели | Паттерн деградации | Порог |
|---|---|---|
| Reasoning (o3, Gemini-2.5-Pro) | Threshold decay | ~150 инструкций, затем резкий обвал |
| Standard (GPT-4.1, Claude Sonnet) | Linear decay | Постепенное снижение по всему диапазону |
| Lightweight (Haiku, Llama-Scout) | Exponential decay | Быстрое падение с 10-50 |

**Для Ralph:** Claude Sonnet 4 показывает **linear decay**. Дублирование контента = удвоение числа воспринимаемых инструкций. При 50 правил × 2 = 100 воспринимаемых — уже в зоне заметной деградации для linear decay модели.

### 2.4 Серийный позиционный эффект (arxiv:2406.15981)

**Primacy dominates:** В 73 из 104 тестов — преимущество начала контекста. Recency effect — вторичен.

**Для B vs C:** Вариант B размещает правила в user prompt (recency zone). Вариант C добавляет копию в system context (primacy zone). Теоретически C покрывает оба полюса. **Но:** при linear decay удвоение инструкций может нивелировать позиционное преимущество.

### 2.5 Claude-специфичное поведение при дублировании

Deep research обнаружил:

> Claude 3 Haiku и Claude 3.7 Sonnet при дублировании длинных промптов показывают **рост латентности** [arxiv:2512.14982].

> Claude Code достигает **92-97% prefix reuse** — система предпочитает **кеш-прогрев** дублированию.

**Вывод:** Архитектура Claude Code оптимизирована для **не-дублирования**. Дублирование через variant C работает ПРОТИВ cache-стратегии Claude Code, потенциально увеличивая latency.

### 2.6 Итог по repetition research

| Аспект | Аргумент за C | Контраргумент | Вес |
|---|---|---|---|
| 47/70 wins | Стабильно полезно | Только для non-reasoning; Ralph = reasoning | НИЗКИЙ |
| 0 losses | Zero downside | При linear decay удвоение = больше инструкций = деградация | СРЕДНИЙ |
| Dual position | Primacy + recency | Lost in Middle не применим при <10K tokens | НИЗКИЙ |
| Latency neutral | В paper: "no latency increase" | Claude-specific: рост latency на длинных промптах | СРЕДНИЙ |

**Net assessment:** Prompt repetition research **не обосновывает** вариант C для Ralph. Gains минимальны (reasoning model), а побочные эффекты (latency, instruction count doubling) — реальны.

---

## 3. Token cost дублирования

### 3.1 Текущий масштаб

Ralph-знания на новом проекте начинаются с 0 и растут:

| Фаза | Правил | Tokens (1x) | Tokens (2x, вариант C) | % от 200K |
|---|---|---|---|---|
| Early (1-30) | ~30 | ~1K | ~2K | 1% |
| Growth (30-100) | ~100 | ~3K | ~6K | 3% |
| Mature (100-300) | ~300 | ~9K | ~18K | 9% |
| Scale (300-500) | ~500 | ~15K | ~30K | **15%** |

### 3.2 Оценка

При <100 правил (1-3K tokens) дублирование = ~3-6K. Это 1.5-3% от 200K — **тривиально**.

При 300-500 правил дублирование = 18-30K tokens. Это 9-15% — **значимо**. По данным R1 [S5], context rot начинается при >20-50K tokens документов. Дублирование при 500 правилах приближает к порогу.

**Но:** При 500 правилах glob-scoped loading загружает только 10-15 релевантных (R2, analyst-5). Дублируются только загруженные, не все 500. Реально = ~1-3K × 2 = 2-6K tokens даже при большой базе.

**Вывод:** Token cost — **не решающий фактор** при любом масштабе. Ни за B, ни за C.

---

## 4. Compliance gain: есть ли реальный выигрыш?

### 4.1 Оценки compliance по каналам (R2, §4.2)

| Канал | Estimated compliance | Framing |
|---|---|---|
| SessionStart hook | ~90-94% | Без disclaimer |
| CLAUDE.md | ~70-80% | "MUST follow exactly" |
| `.claude/rules/` | ~40-60% | "project instructions" (бывший disclaimer) |
| Go injection (user prompt) | ~85-95% | Без framing, recency zone |

### 4.2 Что даёт вариант C

Вариант C добавляет канал `.claude/rules/` (~40-60% compliance) поверх Go injection (~85-95%). Вопрос: **улучшает ли 40-60% канал общий результат 85-95% канала?**

**Сценарий 1: Каналы независимы.** P(miss) = P(miss_injection) × P(miss_rules) = 0.1 × 0.5 = 0.05. Итого: ~95% vs 90% для B alone. Gain: **+5 п.п.**

**Сценарий 2: Каналы коррелированы.** Если модель игнорирует правило, она игнорирует его в обоих каналах (одна и та же attention bottleneck). Gain: **~0 п.п.**

**Реальность ближе к сценарию 2:** Если модель не обращает внимание на правило в user prompt (где оно без disclaimer, в recency zone), добавление того же правила в system context с weaker framing вряд ли поможет.

**Оценка: +0 до +5 п.п. compliance gain.**

### 4.3 Что теряет вариант C

Analyst-2 отмечает: `.claude/rules/` в варианте C получает framing "project instructions" — это лучше чем бывший "may or may not be relevant", но всё ещё **не императивный** "MUST follow" [analyst-2, §3].

Analyst-1 (§7.1) приводит количественные данные: `.claude/rules/` compliance = 40-60%, Go injection = 85-95%. **Разница ~35 п.п.** Это означает: сам по себе канал `.claude/rules/` — ненадёжный. Redundancy через ненадёжный канал — слабая redundancy.

---

## 5. Безопасность (CVE surface)

### 5.1 Известные CVE

| CVE | CVSS | Вектор | Impact на B | Impact на C |
|---|---|---|---|---|
| CVE-2025-59536 | 8.7 | RCE через `.claude/settings.json` hooks | **Нулевой** — .ralph/ не часть .claude/ | **Есть** — ralph-*.md в .claude/ |
| CVE-2026-21852 | — | API credential theft через MCP configs | **Нулевой** | **Есть** |
| CVE-2026-25725 | — | Privilege escalation через `.claude/` manipulation | **Нулевой** | **Есть** |

### 5.2 Анализ

**Вариант B:** `.ralph/rules/` — отдельная директория, контролируемая только Go-кодом Ralph. Атакующий должен модифицировать Go binary или runtime. Attack surface = **код Ralph**.

**Вариант C:** ralph-*.md в `.claude/rules/` — часть доверенного контекста Claude Code. Атакующий через malicious PR (добавляет prompt injection в ralph-testing.md) получает доступ к Claude Code execution context. Attack surface = **код Ralph + экосистема Claude Code**.

**Вывод:** B имеет **строго меньший** attack surface. Это **объективный**, невесовой факт — C не может быть безопаснее B по конструкции.

---

## 6. Сложность реализации и поддержки

### 6.1 Вариант B

```
Go code:
  1. os.ReadDir(".ralph/rules/")
  2. Фильтрация по pattern/budget
  3. strings.ReplaceAll(prompt, "__RALPH_KNOWLEDGE__", content)
```

**Сложность:** ~100-200 LoC. Полностью тестируемо unit-тестами. Один канал = одна точка отладки.

### 6.2 Вариант C

```
Go code:
  1. os.ReadDir(".claude/rules/")        ← те же файлы
  2. Фильтрация по pattern/budget
  3. strings.ReplaceAll(prompt, "__RALPH_KNOWLEDGE__", content)

Claude Code (black box):
  4. Auto-loads .claude/rules/ralph-*.md  ← неконтролируемо
  5. Wraps in system-reminder framing     ← неконтролируемо
  6. Position in context = unknown        ← неконтролируемо
```

**Дополнительная сложность варианта C:**

| Аспект | Описание | Severity |
|---|---|---|
| Тестирование канала 2 | Claude Code auto-load = black box, нет API для проверки | HIGH |
| Namespace management | `.claude/rules/` может содержать файлы пользователя | MEDIUM |
| `.gitignore` решение | Игнорировать ralph-*.md? Тогда зачем auto-load. Не игнорировать? Попадают в PR diff | MEDIUM |
| Bug #16299 | Все ralph-*.md грузятся всегда, даже без Ralph | MEDIUM |
| Sync validation | Go и Claude Code видят одни файлы, но Go не может проверить что Claude Code загрузил | HIGH |

**Вывод:** Вариант C добавляет 50% untestable delivery path без пропорциональной выгоды.

---

## 7. Поведение при росте базы знаний

### 7.1 Масштабирование по фазам

| Фаза | Правил | Вариант B | Вариант C |
|---|---|---|---|
| **0-30** | Start | Go загружает все. Просто. | Go + auto-load. Двойной overhead тривиален. |
| **30-100** | Growth | Go начинает glob-filtering. Budget control. | Go фильтрует, но auto-load грузит ВСЕ (Bug #16299). Conflict. |
| **100-300** | Mature | Go подаёт top-N релевантных (~15). Budget enforced. | Go подаёт 15, Claude Code подаёт ВСЕ. Budget нарушен через второй канал. |
| **300-500** | Scale | Go adaptive budget. Total control. | Go адаптивен, Claude Code загружает 300+ файлов. Context dilution. |

### 7.2 Критическая проблема масштабирования варианта C

При >100 правил Go начинает фильтровать (подавать только релевантные 10-15). **Но Claude Code продолжает загружать ВСЕ ralph-*.md из `.claude/rules/`** — нет механизма сказать Claude Code "загрузи только эти 3 файла из 20."

`paths:` frontmatter мог бы решить это, но **Bug #16299 сломан** — все файлы грузятся безусловно.

**Результат:** При 300+ правилах вариант C теряет ключевое преимущество Go-controlled injection — адаптивный budget. Второй канал становится **source of noise**, загружая все 300+ правил поверх отфильтрованных 15.

**Вариант B не имеет этой проблемы** — единственный канал, полный контроль на любом масштабе.

### 7.3 Количественная оценка (R2, SFEIR)

| Загружено правил | Compliance (per R2) |
|---|---|
| 15 (Go-filtered, B) | ~94% |
| 15 (Go) + ALL (auto-load, C at 300 rules) | ~40-50% (volume ceiling reached) |

При масштабировании **вариант C деградирует**, B — нет.

---

## 8. Совместимость с pipe mode

### 8.1 Вариант B

`claude --print` получает промпт с `__RALPH_KNOWLEDGE__` заменённым на контент. **Гарантированно работает** — это обычная строка в промпте.

### 8.2 Вариант C

`claude --print` должен загрузить `.claude/rules/` автоматически. Документация [code.claude.com/docs/en/memory]:

> "Rules without paths frontmatter are loaded at launch with the same priority as .claude/CLAUDE.md."

Pipe mode (`-p`) = "non-interactive" сессия. С **высокой вероятностью** `.claude/rules/` загружается в pipe mode. Но **100% гарантии нет** — документация не содержит явного утверждения "pipe mode loads .claude/rules/."

Analyst-2 (§2) правильно отмечает: snapshot problem не актуален для `claude --print` (каждый вызов = fresh read). Это корректно и устраняет один аргумент против C.

**Вывод:** Pipe mode — **не блокер** для C, но вносит uncertainty. B не зависит от pipe mode поведения Claude Code.

---

## 9. Что происходит когда Bug #16299 починят

### 9.1 Текущее состояние

Bug #16299: `paths:` frontmatter в `.claude/rules/` **сломан** — все файлы грузятся безусловно, независимо от указанных paths.

### 9.2 Если починят — impact на B

**Нулевой.** B не использует `.claude/rules/`. Починка Bug #16299 не влияет на архитектуру.

### 9.3 Если починят — impact на C

**Позитивный, но ограниченный:**

1. Ralph сможет использовать `paths:` frontmatter для selective loading:
   ```yaml
   ---
   paths:
     - "**/*_test.go"
   ---
   ```
2. Это частично решает проблему масштабирования (§7.2) — Claude Code будет загружать только релевантные ralph-*.md.

**Но остаются:**
- Disclaimer/framing проблема (не зависит от Bug #16299)
- CVE surface (не зависит)
- Untestable channel (не зависит)
- Namespace conflict с пользователем (не зависит)

**Вывод:** Починка Bug #16299 улучшает C, но **не устраняет** фундаментальные проблемы (framing, security, testability). Разрыв между B и C сокращается, но B остаётся лучше.

### 9.4 Сценарий: Anthropic убирает disclaimer

Если `.claude/rules/` получит императивный framing "MUST follow" (как CLAUDE.md):

- Compliance канала 2 вырастет с ~40-60% до ~70-80%
- C становится значительно привлекательнее
- **Но:** Go injection всё равно = ~85-95% (user prompt, recency). Gain от добавления 70-80% канала к 85-95% = +3-7 п.п.
- CVE surface и untestability остаются

**Вывод:** Даже в best-case сценарии (Bug фиксирован + disclaimer убран) вариант C даёт marginal gain при non-trivial cost.

---

## 10. Риски и edge cases

### 10.1 Риски варианта B

| Риск | Severity | Вероятность | Mitigation |
|---|---|---|---|
| Go injection bug = zero knowledge delivery | HIGH | LOW | Unit tests, golden files |
| `.ralph/` директория непривычна пользователю | LOW | MEDIUM | Документация, `ralph init` |
| Нет fallback при ошибке чтения файлов | MEDIUM | LOW | Error handling, graceful degradation |

### 10.2 Риски варианта C

| Риск | Severity | Вероятность | Mitigation |
|---|---|---|---|
| Claude Code меняет framing/loading behavior | HIGH | MEDIUM | Нет контроля |
| Bug #16299 чинят и ломают другое | MEDIUM | LOW | Мониторинг releases |
| Пользователь редактирует ralph-*.md вручную | MEDIUM | MEDIUM | Предупреждение в файле |
| CVE через `.claude/rules/` supply chain | HIGH | LOW | Нет контроля |
| Auto-load + Go injection = instruction count doubling | MEDIUM | HIGH | Нет mitigation (by design) |
| При 300+ правилах auto-load убивает budget | HIGH | MEDIUM (при росте) | Нет mitigation до починки Bug |
| Latency рост на Claude models при дублировании | MEDIUM | MEDIUM | Нет mitigation |

### 10.3 Edge case: пользователь уже имеет `.claude/rules/`

**Вариант B:** `.ralph/rules/` — отдельная директория. Нет конфликта. Clean separation.

**Вариант C:** ralph-*.md добавляются в `.claude/rules/` пользователя. Возможные проблемы:
- Пользователь имеет 20 своих rules + Ralph добавляет 15 = 35 → выше 15-rule compliance threshold
- Пользователь может иметь conflicting rules (его "testing.md" vs ralph "ralph-testing.md")
- `.gitignore` решение: если ralph-*.md в `.gitignore` — они не видны в интерактивном Claude Code (теряется value auto-load). Если не в `.gitignore` — попадают в git history, PR diffs.

---

## 11. Disagreement analysis: analyst-1 vs analyst-2

### 11.1 Почему analyst-1 рекомендует B (confidence 95%)

Analyst-1 строит аргумент на:
1. **Compliance gap ~35 п.п.** между channels (.claude/rules/ = 40-60% vs injection = 85-95%)
2. **Triple barrier** из R2 (compaction, context rot, volume ceiling)
3. **JIT validation incompatibility** — Go не участвует в auto-load pipeline
4. **CVE surface** — строго больше для C

### 11.2 Почему analyst-2 рекомендует C (score 8.05 vs 5.90)

Analyst-2 строит аргумент на:
1. **Prompt repetition** — 47/70 wins, 0 losses
2. **Anthropic guidance** — "instructions in human messages work better"
3. **Dual position** — anti-lost-in-middle
4. **Graceful degradation** — два канала > один

### 11.3 Где каждый прав и неправ

| Аспект | Analyst-1 | Analyst-2 | Мой вердикт |
|---|---|---|---|
| Repetition gains | Недооценивает (называет "минимальным") | Переоценивает (не разделяет reasoning/non-reasoning) | Gains реальны но **нерелевантны** для reasoning mode |
| Lost in Middle | Корректно отмечает неприменимость для instructions | Применяет к instructions <10K tokens | **Analyst-1 прав** — не применимо |
| CVE surface | Корректно — строго больше для C | Упоминает, но не weighted | **Analyst-1 прав** — объективный факт |
| Graceful degradation | Недооценивает | Корректно — два канала > один | **Analyst-2 прав** — redundancy реальна |
| Scalability | Корректно — C деградирует при росте | Не рассматривает масштабирование >100 правил | **Analyst-1 прав** — критический gap в анализе C |
| Testability | Корректно — 50% untestable | Недооценивает (score 9/10 для C testability) | **Analyst-1 прав** — Go может тестировать только свой канал |
| Token cost | Correct but overweighted | Correct — negligible at current scale | **Оба правы**, не решающий фактор |

### 11.4 Methodology critique

**Analyst-2** использует weighted scoring (8.05 vs 5.90), но:
- "Instruction compliance" (25% weight) = 9/10 для C основано на repetition research **без учёта** reasoning mode caveat
- "Context position effectiveness" (20% weight) = 9/10 для C основано на Lost in Middle **неприменимом** к instructions <10K tokens
- "Graceful degradation" (10% weight) = 9/10 для C — **корректно**

Если скорректировать:
- Instruction compliance: C = 7/10 (reasoning mode minimal gain), D = 7/10 → no advantage
- Context position: C = 6/10 (not applicable at <10K), D = 5/10 → minimal advantage

**Скорректированный score: C = ~6.8, D = ~6.15** — gap сокращается до ~0.65 points, что **в пределах погрешности** scoring methodology.

---

## 12. Сводная матрица

| Критерий | Weight | B Score | C Score | Winner | Обоснование |
|---|---|---|---|---|---|
| **Compliance control** | 20% | 9/10 | 7/10 | **B** | Единственный канал = полный контроль. C добавляет ненадёжный второй канал. |
| **Testability** | 20% | 10/10 | 5/10 | **B** | B полностью тестируем. C имеет 50% untestable path. |
| **Security (CVE)** | 15% | 10/10 | 6/10 | **B** | B не использует .claude/. Объективно меньший attack surface. |
| **Scalability** | 15% | 9/10 | 4/10 | **B** | B адаптивен при любом масштабе. C деградирует при >100 правил (auto-load all). |
| **Redundancy** | 10% | 4/10 | 8/10 | **C** | C имеет fallback через второй канал. Единственное реальное преимущество. |
| **Simplicity** | 10% | 9/10 | 6/10 | **B** | B = один канал, одна точка отладки. C = два канала, sync concerns. |
| **Portability** | 5% | 10/10 | 5/10 | **B** | B работает с любым LLM backend. C привязан к Claude Code. |
| **Bug #16299 immunity** | 5% | 10/10 | 3/10 | **B** | B не зависит. C зависит критически для масштабирования. |
| **Weighted Total** | **100%** | **8.55** | **5.65** | **B** | |

---

## 13. Дополнительные находки из deep research

### 13.1 Cache-стратегия Claude Code (новое)

LMCache analysis показывает: Claude Code достигает 92-97% prefix reuse через кеширование повторяющегося системного промпта. Дублирование контента через два канала **работает против** этой cache-стратегии — два разных представления одного контента (system-reminder vs user prompt) не кешируются вместе.

### 13.2 IFScale: instruction scaling (новое)

Бенчмарк IFScale (2025) показывает три паттерна деградации. Claude Sonnet = **linear decay**. Удвоение числа воспринимаемых инструкций (через дублирование) сдвигает по кривой деградации. При 50 правил × 2 = 100 perceived instructions — compliance ~60% вместо ~80%.

### 13.3 Anthropic: "кеш-прогрев > дублирование" (новое)

Deep research выявил: Claude Code architecture **предпочитает** кеш-прогрев (один большой промпт кешируется и переиспользуется) дублированию. Субагенты получают **только role-specific контекст**, без дублирования мастер-промпта. Это design decision Anthropic **против** дублирования.

### 13.4 Optimal rules count (новое)

Arize AI / Cline / SWE-bench: оптимальный объём = **20-50 правил** для лучшего баланса. Не 100 × 2. Дублирование увеличивает perceived rule count, сдвигая от оптимума.

---

## 14. Рекомендация

### Основная: Вариант B (`.ralph/rules/` + Go injection)

**Обоснование по приоритету:**

1. **Testability** (критично для quality): B = 100% testable pipeline. C = 50% black box.
2. **Scalability** (критично для growth): B адаптивен при 0-500+ правил. C деградирует при >100.
3. **Security**: B имеет строго меньший CVE surface.
4. **Simplicity**: один канал = одна точка ответственности.
5. **Independence**: не зависит от Bug #16299, disclaimer framing, Claude Code behavior changes.

### Когда пересмотреть:

Вариант C становится обоснован **только при совпадении ВСЕХ условий:**
- Bug #16299 починен (paths: работает)
- Disclaimer заменён на императивный framing
- Empirical A/B тест показывает measurable compliance gain
- Knowledge base стабилизировалась на <100 правил

### Smart repetition alternative

Вместо полного дублирования через два канала (вариант C), Go может реализовать **selective repetition** в рамках единственного канала (вариант B):

```
__RALPH_KNOWLEDGE__:
  [All relevant rules]
  ...
  === CRITICAL REMINDERS ===
  [Top 3-5 rules repeated at the end of prompt]
```

Это даёт recency-эффект от repetition research **без** second-channel overhead. Полностью тестируемо, контролируемо, безопасно.

---

## 15. Confidence и ограничения

**Confidence: 88% за B.**

Не 95% (как analyst-1), потому что:
- Redundancy advantage C реальна (§12, 8/10 score)
- Если Anthropic улучшит framing `.claude/rules/`, gap сократится
- Empirical A/B testing не проводился — все оценки compliance теоретические

Не ниже 85%, потому что:
- Scalability argument объективен и количественен
- CVE surface объективен
- Testability объективна
- Deep research findings (IFScale, cache strategy) усиливают B

---

## Источники

### Академические (Tier A)

1. [Prompt Repetition Improves Non-Reasoning LLMs](https://arxiv.org/abs/2512.14982) — Google Research, 2025
2. [Lost in the Middle: How LLMs Use Long Contexts](https://arxiv.org/abs/2307.03172) — TACL 2024
3. [Serial Position Effects of LLMs](https://arxiv.org/abs/2406.15981) — 2024
4. [How Many Instructions Can LLMs Follow at Once?](https://arxiv.org/abs/2507.11538) — IFScale, 2025
5. [Found in the Middle: Calibrating Positional Attention Bias](https://arxiv.org/abs/2406.16008) — ACL Findings 2024

### Документация и инженерные отчёты (Tier A/B)

6. [Claude Code Memory docs](https://code.claude.com/docs/en/memory)
7. [Claude Code Headless/Pipe Mode docs](https://code.claude.com/docs/en/headless)
8. [Anthropic Claude Prompting Best Practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)
9. [Anthropic: Effective Context Engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
10. [LMCache: Context Engineering in Claude Code](https://blog.lmcache.ai/en/2025/12/context-engineering-reuse-pattern-under-the-hood-of-claude-code/)
11. [Anthropic/OpenAI Alignment Evaluation](https://alignment.anthropic.com/2025/openai-findings/)
12. [Arize AI: Optimizing Coding Agent Rules](https://arize.com/blog/optimizing-coding-agent-rules-claude-md-agents-md-clinerules-cursor-rules-for-improved-accuracy/)

### CVE / Security (Tier A)

13. [CVE-2025-59536: RCE via Claude Code Project Files](https://research.checkpoint.com/2026/rce-and-api-token-exfiltration-through-claude-code-project-files-cve-2025-59536/)
14. [CVE-2026-21852: API credential theft](https://thehackernews.com/2026/02/claude-code-flaws-allow-remote-code.html)
15. [CVE-2026-25725: Privilege escalation](https://www.sentinelone.com/vulnerability-database/cve-2026-25725/)
16. [Bug #16299: Path-scoped rules load globally](https://github.com/anthropics/claude-code/issues/16299)

### Предыдущие отчёты проекта

17. `docs/reviews/v2-analyst-1-ab-report.md` — Analyst-1, рекомендует B (confidence 95%)
18. `docs/reviews/v2-analyst-2-cd-report.md` — Analyst-2, рекомендует C (score 8.05 vs 5.90)
19. `docs/reviews/v2-analyst-8-synthesis-report.md` — Мета-анализ
20. `docs/research/knowledge-extraction-in-claude-code-agents.md` — R1 (20 источников)
21. `docs/research/knowledge-enforcement-in-claude-code-agents.md` — R2 (40 источников)
22. `docs/research/alternative-knowledge-methods-for-cli-agents.md` — R3 (22 источников)
