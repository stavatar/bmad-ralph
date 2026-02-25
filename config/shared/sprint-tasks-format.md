# Sprint Tasks Format Specification

You MUST follow this exact format when generating or parsing `sprint-tasks.md` files.

## Section Headers

Group tasks under epic headers using level-2 markdown headings:

```
## Epic Name
```

Example:

```
## Authentication & Security
## Data Pipeline
## Admin Dashboard
```

## Task Syntax

### Open (incomplete) task

```
- [ ] Task description
```

The marker `- [ ]` (dash, space, open bracket, space, close bracket) marks an incomplete task.

### Completed task

```
- [x] Task description
```

The marker `- [x]` (dash, space, open bracket, lowercase x, close bracket) marks a completed task.

### Gate task (requires human approval)

Append `[GATE]` tag at the end of the task line:

```
- [ ] Deploy to staging [GATE]
```

Gates pause automated execution and require explicit human approval before proceeding.

## Source Traceability

Every task MUST have an indented `source:` field on the line immediately following the task line. The source field traces the task back to its origin story and acceptance criterion.

Format:

```
  source: stories/<filename>.md#<identifier>
```

Rules:
- The `source:` line MUST be indented (at least one space or tab) under its parent task
- The path and identifier are separated by `#`
- The identifier after `#` MUST be non-empty

Examples:

```
- [ ] Implement JWT token validation
  source: stories/1-2-user-auth.md#AC-3
- [x] Add rate limiting middleware
  source: stories/1-5-api-security.md#AC-1
- [ ] Configure SSL certificates [GATE]
  source: stories/2-1-deployment.md#AC-2
```

Source field regex pattern: `^\s+source:\s+\S+#\S+`

## Service Task Prefixes

Service tasks are infrastructure or validation tasks not directly tied to a user story AC. Prefix the task description with one of these tags:

| Prefix | Purpose | Example |
|--------|---------|---------|
| `[SETUP]` | Environment or infrastructure setup | `- [ ] [SETUP] Initialize database migrations` |
| `[VERIFY]` | Verification or validation step | `- [ ] [VERIFY] Run integration test suite` |
| `[E2E]` | End-to-end test or workflow | `- [ ] [E2E] Complete user registration flow` |

Service tasks still follow the same `source:` traceability format:

```
- [ ] [SETUP] Create staging environment
  source: stories/2-1-deployment.md#SETUP
- [ ] [VERIFY] Validate API contract compliance
  source: stories/1-3-api-design.md#VERIFY
- [ ] [E2E] Test checkout flow end-to-end
  source: stories/3-2-checkout.md#E2E
```

## User Feedback

When a human gate reviewer provides feedback, it appears as a blockquote prefixed with `> USER FEEDBACK:` immediately after the relevant task:

```
- [ ] Deploy to staging [GATE]
  source: stories/2-1-deployment.md#AC-2
> USER FEEDBACK: Staging deploy needs VPN configuration first
```

## Complete Example

```markdown
## Authentication & Security

- [x] Implement password hashing with bcrypt
  source: stories/1-1-user-model.md#AC-2
- [x] Add JWT token generation
  source: stories/1-2-user-auth.md#AC-1
- [ ] Implement JWT token validation
  source: stories/1-2-user-auth.md#AC-3
- [ ] Add refresh token rotation [GATE]
  source: stories/1-2-user-auth.md#AC-4

## API Infrastructure

- [x] [SETUP] Configure API rate limiter
  source: stories/1-5-api-security.md#SETUP
- [ ] [VERIFY] Validate rate limit thresholds
  source: stories/1-5-api-security.md#VERIFY
- [ ] Implement pagination for list endpoints
  source: stories/1-3-api-design.md#AC-2
> USER FEEDBACK: Use cursor-based pagination, not offset

## Deployment

- [ ] [E2E] Full deployment pipeline test
  source: stories/2-1-deployment.md#E2E
- [ ] Configure production monitoring [GATE]
  source: stories/2-1-deployment.md#AC-5
```
