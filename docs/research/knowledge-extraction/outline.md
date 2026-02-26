# Report Outline: Knowledge Extraction in Claude Code-Based Coding Agents

## Title
Knowledge Extraction in Claude Code-Based Coding Agents: Patterns, Limitations, and Architectural Recommendations

## Executive Summary
- 3-6 bullets covering: state of knowledge management, what works, what doesn't, context rot implications, recommendations for bmad-ralph

## Research Question and Scope
- Primary: How to effectively extract, store, and apply knowledge in coding agents based on Claude Code?
- Sub-questions: memory mechanisms, best practices, context rot, effectiveness data, architectural patterns
- Scope: Claude Code ecosystem, GitHub Copilot for comparison, 2024-2026
- Exclusions: non-LLM memory systems, Cursor/Copilot internal architecture

## Methodology
- Web research, official documentation, engineering blog posts, academic research
- Source quality: Tier A (Anthropic/GitHub official), Tier B (reputable blogs), excluded Tier C

## Key Findings
- 5-10 numbered findings with citations

## Analysis

### Section 1: Claude Code Memory Architecture
- 6 memory types and hierarchy [S4]
- Auto memory: 200-line limit, topic files [S4]
- CLAUDE.md cascade and loading order [S4]
- Skills and rules directories [S12, S13]

### Section 2: Context Rot and Attention Limitations
- Universal degradation across 18 models [S5]
- 30-50% performance variance compact vs full context [S5]
- ~150-200 instruction following limit [S14]
- "Lost in the middle" effect [S5]
- Framing problem: CLAUDE.md disclaimer wrapper [S6]

### Section 3: Knowledge Persistence Patterns
- Compaction: capabilities and limitations [S1, S2]
- Structured note-taking (NOTES.md, progress files) [S1, S2]
- Just-in-time retrieval vs pre-loading [S2]
- Hook-based enforcement [S6, S13]
- Sub-agent architecture for context isolation [S2]

### Section 4: Third-Party and Comparative Approaches
- GitHub Copilot agentic memory: citation-based validation, 7% improvement [S3, S18]
- claude-mem: automatic capture, semantic summarization [S10]
- Claudeception: autonomous skill extraction [S9]
- Continuous-learning skill pattern [S11]

### Section 5: Implications for bmad-ralph Knowledge System
- Current design: LEARNINGS.md (200 lines) + CLAUDE.md section
- Evidence-based assessment of what will work
- Structural recommendations: topic files, citation validation, contextual filtering
- Distillation as compression: risks and mitigations
- Hook-based enforcement for critical rules

## Risks and Limitations
- Context rot is universal and unsolved
- Auto memory accuracy issues
- No comprehensive effectiveness benchmarks
- Copilot data from different architecture

## Recommendations
- 5-7 actionable recommendations for bmad-ralph Epic 6

## Appendix A: Evidence Table
## Appendix B: Sources
