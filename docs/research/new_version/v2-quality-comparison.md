# Сравнение качества декомпозиции: Bridge (Stories -> Tasks) vs Прямая (PRD -> Tasks)

Дата: 2026-03-07
Проект-источник: learnPracticsCodePlatform (TypeScript/NestJS/React)

---

## 1. Аудит качества sprint-tasks.old.md (результат Bridge)

### 1.1 Общая статистика

| Метрика | Значение |
|---------|----------|
| Всего задач | 93 |
| Эпиков | 9 |
| Source-файлов (уникальных) | 34 |
| Задач с source | 93 (100%) |
| Задач без source | 0 |
| Дубликатов | 0 |

### 1.2 Распределение задач по эпикам

| Эпик | Задач |
|------|-------|
| Epic 0: Infrastructure & Foundation | 15 |
| Epic 1: Authentication & Security | 10 |
| Epic 2: Sandbox Pipeline | 10 |
| Epic 3: Editor & Task Flow | 10 |
| Epic 4: Catalog & Progress | 9 |
| Epic 5: Admin — Создание задач | 11 |
| Epic 6: AI Generation Pipeline | 9 |
| Epic 7: Batch Import | 13 |
| Epic 8: Mobile Stub | 6 |

### 1.3 Распределение по размеру (количество символов)

```
<50:      0  |
50-99:    1  |#
100-199:  5  |#####
200-299: 11  |###########
300-499: 23  |#######################
>500:    53  |#####################################################
```

| Метрика | Значение |
|---------|----------|
| Минимум | 61 символов |
| Максимум | 1636 символов |
| Среднее | 633 символа |
| Медиана | ~500 символов |

**Проблема: 57% задач (53 из 93) превышают 500 символов.** Это мегазадачи, каждая из которых содержит несколько независимых шагов, упакованных в один элемент списка. Примеры:

- Задача на 1636 символов (OnboardingOverlay) — по сути 5-7 отдельных подзадач
- Задача на 1420 символов (useAdminStats) — описывает полный хук с 4 API-вызовами
- Задача на 1293 символов (DraftsPage) — целая страница с роутингом, фильтрами, группировкой

### 1.4 Source-ссылки

#### Префикс `stories/`

Все 93 source-ссылки используют несуществующий префикс `stories/`:
```
source: stories/0-1-monorepo-scaffold.md#build
```
Реальный путь: `docs/sprint-artifacts/0-1-turborepo-monorepo-scaffold.md`

**Две проблемы:**
1. Директория `stories/` не существует — bridge использует свой внутренний маппинг
2. Имена файлов сокращены/искажены у 23 из 34 файлов

#### Некорректные имена файлов (23 из 34)

| source-ссылка | Реальный файл |
|---------------|---------------|
| `stories/0-1-monorepo-scaffold.md` | `0-1-turborepo-monorepo-scaffold.md` |
| `stories/0-2-prisma-schema.md` | `0-2-prisma-schema-postgresql-setup.md` |
| `stories/1-1-auth-module.md` | `1-1-nestjs-auth-module-jwt-guards.md` |
| `stories/2-1-sandbox-pool.md` | `2-1-sandbox-pool-warm-container-management.md` |
| `stories/2-3-submission-queue.md` | `2-3-bullmq-submission-queue.md` |
| `stories/3-1-monaco-editor.md` | `3-1-monaco-editor-setup-java-support.md` |
| ... | (ещё 17 пар) |

Только 11 из 34 файлов совпадают (32% точность имён).

#### Якоря (`#fragment`)

Якоря в source-ссылках — произвольные, не совпадают ни с заголовками Tasks в story, ни с AC-номерами. Например:
- `#build` — не соответствует `### Task 1: Initialize root monorepo`
- `#client` — не соответствует `### Task 2: Scaffold apps/client`

Якоря выглядят как самодельная маркировка bridge, а не реальные ссылки на разделы документа.

### 1.5 [GATE] расстановка

Всего 9 задач с [GATE]:

| Задача | Корректность |
|--------|-------------|
| Initialize root monorepo | OK — фундамент для всего |
| Create PrismaModule | OK — база для всех модулей |
| Seccomp profile | СПОРНО — это часть sandbox story, не обязательно gate |
| Install @monaco-editor/react [SETUP][GATE] | СПОРНО — SETUP+GATE одновременно неоднозначно |
| TaskListItemDto interface | OK — контракт для каталога |
| Admin task DTOs | OK — контракт для CRUD |
| Install @mentorlearn/gemini-cli [SETUP][GATE] | OK — внешняя зависимость |
| Import module skeleton | OK — каркас для импорта |
| useMediaQuery hook | СПОРНО — маленький хук не заслуживает gate |

**Вердикт:** 5/9 однозначно корректны, 4 спорных. Нет грубых ошибок, но есть шум.

### 1.6 [SETUP] без [GATE]

6 задач [SETUP] без [GATE]:
- Install dockerode
- Install shadcn/ui components (x2)
- Install @nestjs/bullmq
- Install react-markdown
- Install @nestjs/schedule

**Проблема:** Эти задачи — чистые `npm install`, которые:
1. Не требуют творческих решений
2. Не имеют AC
3. Могут быть частью первой задачи, которая использует зависимость

### 1.7 Сводка проблем

| Проблема | Серьёзность | Количество |
|----------|-------------|------------|
| Слишком крупные задачи (>500 симв.) | ВЫСОКАЯ | 53 (57%) |
| Неправильные имена файлов в source | СРЕДНЯЯ | 23 (68%) |
| Несуществующий префикс `stories/` | СРЕДНЯЯ | 93 (100%) |
| Якоря не ссылаются на реальные разделы | НИЗКАЯ | 93 (100%) |
| [SETUP] без [GATE] как отдельные задачи | НИЗКАЯ | 6 |
| Выдуманные файлы | НЕТ | 0 |
| Дубликаты | НЕТ | 0 |
| Задачи без source | НЕТ | 0 |

---

## 2. Что теряется от Stories к Tasks

### 2.1 Story 0-1 (Monorepo Scaffold): 6 Tasks -> 5 Tasks

**Потерянное:**
- Task 6 "Verify full build" (turbo build, lint, typecheck) — задача верификации полностью исчезла
- AC на `npx turbo build` — не покрыт отдельной задачей

**Dev Notes -> Tasks (покрытие 11/15 = 73%):**

| Элемент Dev Notes | В задачах? |
|-------------------|-----------|
| Turborepo npm workspaces | Да |
| React 19 + Vite 6 | Да |
| TypeScript strict:true | Да |
| NestJS 11 | Да |
| **Tailwind CSS v4** | **НЕТ** |
| **shadcn/ui** | **НЕТ** |
| Vitest config | Да |
| **Jest (NestJS default)** | **НЕТ** |
| ESLint 9 flat config | Да |
| Prettier | Да |
| **Named exports (Architecture Pattern #1)** | **НЕТ** |
| No barrel files (H8) | Да |
| @learnpractics/shared | Да |
| @shared/* paths | Да |

Потеряно 4 элемента: Tailwind CSS, shadcn/ui, Jest, Named exports pattern.

### 2.2 Story 1-1 (Auth Module): 12 Tasks -> 2 Tasks

Экстремальная компрессия: 12 детальных задач сжаты в 2 мегазадачи.

**Задача 1 в sprint-tasks:** PrismaModule (1 из 12 задач story — корректно)

**Задача 2 в sprint-tasks:** ВСЁ ОСТАЛЬНОЕ — AuthModule, DTO, Service, Strategy, Guards, Controller, env validation, тесты — в одной гигантской задаче на ~800 символов.

**Потери:**
- Разделение на unit tests и E2E tests — утеряно (слиты)
- JWT Strategy как отдельный компонент — утерян (внутри мега-задачи)
- Task 7b (GET /api/auth/me, POST /api/auth/logout) — упомянуто, но не как отдельная задача
- Env validation (JWT_SECRET, INVITE_TOKEN) — упомянуто в тексте, но не как шаг
- CORS configuration — утеряно полностью

### 2.3 AC coverage (Story 1-1)

| AC | В задачах sprint-tasks? |
|----|------------------------|
| AC-1: Student join new user | Да (в мега-задаче) |
| AC-2: Returning user | Да (upsert mention) |
| AC-3: Nickname uniqueness | Да (@@unique mention) |
| AC-4: JwtAuthGuard | Да (в мега-задаче) |
| AC-5: RolesGuard | Да (в мега-задаче) |
| AC-6: Invalid/expired JWT | Неявно |
| AC-7: Invalid invite token | Да (validate inviteToken) |

Формально все AC упомянуты, но детали реализации потеряны. Агент, получив мега-задачу, должен сам восстановить всю декомпозицию на JWT Strategy, Guards, Controller и т.д.

### 2.4 Шаблон потерь при Bridge-компрессии

1. **Тестовые задачи** — unit и E2E тесты либо сливаются, либо исчезают
2. **Verification/verify задачи** — проверочные шаги теряются
3. **Env/config задачи** — мелкие конфигурационные задачи поглощаются
4. **Architectural constraints** (Dev Notes) — паттерны вроде "Named exports" не попадают в задачи
5. **Dependencies to install** — секция Dev Notes полностью игнорируется
6. **Project structure notes** — не попадают в задачи

---

## 3. Гипотетическая прямая декомпозиция (PRD + Architecture -> Tasks)

### 3.1 Что доступно в PRD

PRD содержит:
- 51 функциональное требование (FR1-FR51)
- 5 пользовательских journey с конкретными сценариями
- MVP scope с приоритезацией
- NFR с конкретными метриками (30 сек таймаут, 256 MB, 100 учеников)
- Risk mitigation strategy

### 3.2 Что доступно в Architecture

Architecture содержит:
- Полный tech stack с версиями
- Prisma schema со всеми 6 моделями
- Sandbox security с конкретными Docker-флагами
- API endpoints с DTOs
- Project structure с файловыми путями
- Enforcement Guidelines (H1-H8)
- Готовые паттерны (NestJS modules, React lazy loading)

### 3.3 Что получилось бы при прямой декомпозиции

При подаче PRD + Architecture напрямую Claude мог бы:

**Преимущества прямой декомпозиции:**

1. **Цельный контекст:** Агент видит ВСЕ FR одновременно и может группировать задачи оптимально, а не по границам stories
2. **Architecture details доступны напрямую:** Docker-флаги, Prisma schema, API contracts — не нужен промежуточный слой
3. **Cross-cutting concerns видны:** Auth нужен для Catalog, Catalog нужен для Editor — зависимости очевидны
4. **Тестовая стратегия единая:** Можно спланировать unit/E2E/integration тесты системно

**Потери без stories:**

1. **Dev Notes** — stories содержат конкретные решения ("Destroy-not-reuse для sandbox", "OnApplicationShutdown, NOT OnModuleDestroy"), которых нет в PRD/Architecture
2. **Subtask-level decomposition** — stories уже содержат задачи уровня "Create file X with method Y", PRD содержит только FR-уровень
3. **Dependencies to install** — stories явно перечисляют npm-пакеты, PRD нет
4. **Testing requirements** — stories описывают конкретные тест-кейсы, PRD только "автопроверка работает"

### 3.4 Вердикт

Прямая декомпозиция PRD+Architecture может дать **лучше структурированные задачи** (правильный размер, нет мега-задач), но **потеряет implementation details** из Dev Notes. Оптимум — давать PRD+Architecture+Dev Notes (извлечённые из stories или написанные отдельно).

Ключевое наблюдение: **Bridge не извлекает Dev Notes из stories**. Он работает только с AC и Tasks, игнорируя самую ценную часть story — конкретные технические решения.

---

## 4. Метрики качества

### 4.1 Task Clarity (понятно ли что делать)

| Показатель | Оценка | Комментарий |
|------------|--------|-------------|
| Bridge (sprint-tasks.old.md) | **5/10** | Мегазадачи на 1000+ символов перегружены. "Создай X, Y, Z, и ещё A, B, C" — непонятно, с чего начинать |
| Stories (оригиналы) | **9/10** | Каждый Task/Subtask — конкретный шаг с AC-ссылкой |
| Гипотетическая прямая | **7/10** | Зависит от промпта, но FR-уровень достаточно конкретен |

### 4.2 Task Atomicity (одна задача = одна сессия)

| Показатель | Оценка | Комментарий |
|------------|--------|-------------|
| Bridge | **3/10** | 57% задач >500 символов — это НЕ атомарные задачи. Story 1-1: 12 task -> 2 задачи |
| Stories | **8/10** | Tasks хорошо разделены, но некоторые (T3 с 3.3-3.5) группируют |
| Гипотетическая прямая | **6/10** | Claude склонен к умеренным задачам, но без stories нет ориентира на размер |

### 4.3 Task Completeness (все requirements покрыты)

| Показатель | Оценка | Комментарий |
|------------|--------|-------------|
| Bridge | **6/10** | Формально 93 задачи покрывают 34 story, но verification/test задачи теряются |
| Stories | **9/10** | Детальные Tasks с AC-привязкой, Dev Notes, Project Structure |
| Гипотетическая прямая | **7/10** | FR покрыты, но implementation details отсутствуют |

### 4.4 Source Accuracy (source-ссылки корректны)

| Показатель | Оценка | Комментарий |
|------------|--------|-------------|
| Bridge | **2/10** | 100% используют неверный путь `stories/`, 68% имён файлов искажены, якоря произвольные |

### 4.5 Size Distribution (гистограмма)

```
Идеал для Claude Code sessions (~200-400 символов):

sprint-tasks.old.md:
  <200:   ██████ (6)              — слишком мелкие (SETUP)
  200-400: ████████████████ (34)   — оптимальный размер
  >400:   ████████████████████████████████████████████████████ (53) — слишком крупные

Оценка распределения: 37% в оптимальном диапазоне, 57% перегружены
```

---

## 5. Итоговые выводы

### 5.1 Bridge создаёт задачи с тремя системными проблемами

1. **Компрессия:** Story с 12 Tasks сжимается в 2 мега-задачи. Bridge агрессивно мержит, теряя атомарность
2. **Source fidelity:** 68% имён файлов неточны, 100% путей несуществующие, якоря произвольные
3. **Dev Notes blindness:** Самая ценная часть story (конкретные технические решения, версии пакетов, паттерны архитектуры) полностью игнорируется

### 5.2 Stories остаются ценным промежуточным артефактом

Stories содержат то, чего нет ни в PRD, ни в Architecture:
- Конкретные npm-команды и версии
- Решения архитектурных дилемм (OnApplicationShutdown vs OnModuleDestroy)
- Тестовые кейсы привязанные к AC
- File tree что создавать/модифицировать

### 5.3 Рекомендация

**Оптимальная стратегия:** PRD + Architecture -> Tasks напрямую, но с "Dev Hints" — компактными техническими решениями, которые сейчас живут в Dev Notes stories.

Это позволит:
- Избежать потерь при Bridge-компрессии
- Сохранить implementation details
- Получить атомарные задачи правильного размера
- Иметь точные source-ссылки (на PRD FR, Architecture sections)

### 5.4 Количественное сравнение

| Метрика | Bridge | Stories | PRD Direct |
|---------|--------|---------|------------|
| Task Clarity | 5/10 | 9/10 | 7/10 |
| Task Atomicity | 3/10 | 8/10 | 6/10 |
| Task Completeness | 6/10 | 9/10 | 7/10 |
| Source Accuracy | 2/10 | N/A | 8/10 |
| **Среднее** | **4.0** | **8.7** | **7.0** |

Bridge-задачи уступают обоим альтернативам. Stories дают наилучшее качество, но их создание — отдельный шаг. Прямая декомпозиция из PRD — золотая середина при условии включения Dev Hints.
