# FR Coverage Map

| FR | Epic | Stories (planned) |
|----|:----:|-------------------|
| FR1 | 2 | Bridge prompt, Bridge logic |
| FR2 | 2 | Bridge prompt (AC-derived tests) |
| FR3 | 2 | Bridge prompt (gate marking) |
| FR4 | 2 | Smart Merge |
| FR5 | 2 | Bridge prompt (service tasks) |
| FR5a | 2 | Bridge prompt (source traceability) |
| FR6 | 3 | Git client (health check), Runner loop |
| FR7 | 3 | Runner loop (fresh session) |
| FR8 | 3 | Execute prompt, Runner loop (commit detection) |
| FR9 | 3 | Retry logic, Resume-extraction |
| FR10 | 3 | Config (max_turns), Session (--max-turns flag) |
| FR11 | 3 | Execute prompt (self-directing), Scanner |
| FR12 | 3 | Runner loop (resume), Git client (dirty recovery) |
| FR13 | 4 | Review integration в runner loop |
| FR14 | 4 | Review session (fresh session) |
| FR15 | 4 | Review prompt + 4 sub-agent prompts |
| FR16 | 4 | Findings verification |
| FR17 | 4+6 | Clean review / Findings write (Epic 4). Lessons → LEARNINGS.md + CLAUDE.md deferred to Epic 6 via KnowledgeWriter |
| FR18 | 4 | Execute→review loop в runner |
| FR18a | 4 | Review cycle counter в runner |
| FR20 | 5 | Basic gate prompt |
| FR21 | 5 | Gate detection в runner loop |
| FR22 | 5 | Retry with feedback |
| FR23 | 3 | Emergency gate (execute attempts) — safety mechanism |
| FR24 | 3+4 | Emergency gate (review iterations) — safety mechanism |
| FR25 | 5 | Checkpoint gates |
| FR26 | 6 | CLAUDE.md section management |
| FR27 | 6 | LEARNINGS.md append + budget |
| FR28 | 6 | Resume-extraction knowledge writing |
| FR28a | 6 | Review lessons writing + distillation |
| FR28b | 6 | --always-extract |
| FR29 | 6 | Knowledge loading в session context |
| FR30 | 1 | Config struct + YAML parsing |
| FR31 | 1 | CLI flags override |
| FR32 | 1 | Config fallback chain (agent files) |
| FR33 | 1 | Fallback chain (project → global → embedded) |
| FR34 | 1 | Per-agent model config |
| FR35 | 1 | Exit code types + mapping |
| FR36 | 3 | Execute prompt (999-rules) |
| FR37 | 3+4 | Execute prompt (ATDD) + test-coverage agent |
| FR38 | 3+4 | Execute prompt (zero skip) + test-coverage agent |
| FR39 | 6 | Serena integration |

---
