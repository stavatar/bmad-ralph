# Research 5: Human Gate точки в Ralph Loop

**Дата:** 2026-02-24
**Вопрос:** Какие задачи по умолчанию должны иметь human_gate?

---

## 1. Проблема

Ralph loop = автономная итерация. Но некоторые действия требуют человеческого решения. Нужно определить дефолтные точки остановки и механизм.

## 2. Принципы из сообщества

### 2.1 Ключевой вопрос (Permit.io):
> "Would I be okay if the agent did this without asking me?" — если нет, вставь gate.

### 2.2 Когда нужен human gate (консенсус)

| Категория | Примеры | Почему |
|-----------|---------|--------|
| **Необратимые действия** | Delete, drop table, force push | Нельзя откатить |
| **Security-sensitive** | Auth, payments, encryption | Баги = уязвимости |
| **Архитектурные решения** | Новая зависимость, schema change, API contract | Долгосрочные последствия |
| **External-facing** | API endpoints, email sending, webhooks | Видимо пользователям |
| **Subjective quality** | UI/UX, copy, branding | Автотесты не покроют |
| **Cost/resource** | Подключение внешних сервисов, cloud resources | Деньги |

### 2.3 Ralph-специфичные рекомендации

[Community](https://www.leanware.co/insights/ralph-wiggum-ai-coding):
- Не использовать Ralph для auth, payments, data handling без review
- Устанавливать checkpoint intervals для длинных сессий
- `--max-iterations` для предотвращения runaway

[Anthropic best practices](https://skywork.ai/blog/claude-code-2-0-checkpoints-subagents-autonomous-coding/):
- Human-in-the-loop для code review, test execution, approvals before finalize

### 2.4 Четыре паттерна реализации (HITL)

| Паттерн | Как работает | Подходит для Ralph? |
|---------|-------------|:---:|
| **Interrupt & Resume** | Агент останавливается, ждёт input, продолжает | Да — loop pause |
| **Human-as-a-Tool** | Агент вызывает "спросить человека" как инструмент | Нет — fresh context |
| **Approval Flows** | Role-based gates (RBAC) | Overkill для solo |
| **Fallback Escalation** | Уведомление в Slack/email | Post-MVP |

## 3. Дефолтные Human Gate точки для bmad-ralph

### 3.1 По типу задачи

| Тип задачи | human_gate? | Обоснование |
|------------|:-----------:|-------------|
| Обычная реализация | Нет | Автотесты достаточны |
| Первая задача epic'а | **Да** | Верификация направления |
| **Задача, после которой появляется usable UI** | **Да** | Новая функция/экран/кнопка — человек проверяет через интерфейс |
| Последняя задача epic'а | **Да** | Финальный review перед merge |
| Config / env setup | Нет | Low risk |
| Documentation | Нет | Low risk |

**Ключевой принцип:** human gate ставится НЕ по security-risk, а по **user-visible milestone** — когда после задачи появляется что-то, что можно потрогать через интерфейс (новая кнопка, экран, набор экранов, работающий flow).

### 3.2 По фазе спринта

| Момент | human_gate? | Что делает человек |
|--------|:-----------:|-------------------|
| После bridge (генерация sprint-tasks.md) | **Да** | Проверить задачи, приоритеты, scope |
| Каждые N итераций (checkpoint) | **Опционально** | Обзор прогресса |
| После review-фазы с критическими замечаниями | **Да** | Решить fix или accept |
| Перед merge в main | **Да** | Финальное подтверждение |
| При circuit breaker OPEN | **Да** | Разобраться почему застрял |

### 3.3 Формат в sprint-tasks.md

```markdown
- [ ] TASK-1: Implement user authentication
  human_gate: true
  gate_reason: "Security-sensitive: auth module requires human review"

- [ ] TASK-2: Add user profile page

- [ ] TASK-3: Database migration - add roles table
  human_gate: true
  gate_reason: "Irreversible: database schema change"

- [ ] SERVICE: Checkpoint review
  human_gate: true
  gate_reason: "Periodic progress review"
```

## 4. Механизм реализации в loop.sh

### MVP: Простая проверка

```bash
# После успешной итерации, перед следующей:
NEXT_TASK=$(grep -m1 '^\- \[ \]' sprint-tasks.md)

if echo "$NEXT_TASK" | grep -q 'human_gate: true'; then
  GATE_REASON=$(echo "$NEXT_TASK" | grep 'gate_reason:' | sed 's/.*gate_reason: "\(.*\)"/\1/')
  echo "🚦 HUMAN GATE: $GATE_REASON"
  echo "Press Enter to continue or Ctrl+C to stop..."
  read
fi
```

### Production: Notification + timeout

```bash
if is_human_gate "$NEXT_TASK"; then
  notify_user "Human gate reached: $GATE_REASON"  # Slack/email/desktop
  wait_for_approval --timeout 3600                  # 1 hour timeout
fi
```

## 5. Рекомендация

### MVP: 4 обязательных gate'а

1. **После bridge** — review sprint-tasks.md
2. **Первая задача epic'а** — верификация направления
3. **User-visible milestones** — после задач, где появляется usable UI (новая функция, экран, кнопка, работающий flow)
4. **Перед merge** — финальное подтверждение

Код-конвертер размечает `human_gate: true` на задачах, после которых пользователь получает новый интерактивный элемент. Определяется из контекста story (UI-компоненты, user flows, feature completion).

### Production: + checkpoint intervals + notifications

## Источники

- [Human-in-the-Loop Best Practices (Permit.io)](https://www.permit.io/blog/human-in-the-loop-for-ai-agents-best-practices-frameworks-use-cases-and-demo)
- [Human-on-the-Loop evolution (ByteBridge)](https://bytebridge.medium.com/from-human-in-the-loop-to-human-on-the-loop-evolving-ai-agent-autonomy-c0ae62c3bf91)
- [Transparent AI with Human Gates (MarkTechPost)](https://www.marktechpost.com/2026/02/19/how-to-build-transparent-ai-agents-traceable-decision-making-with-audit-trails-and-human-gates/)
- [Ralph Loop checkpoints](https://www.leanware.co/insights/ralph-wiggum-ai-coding)
- [Claude Code checkpoints](https://skywork.ai/blog/claude-code-2-0-checkpoints-subagents-autonomous-coding/)
