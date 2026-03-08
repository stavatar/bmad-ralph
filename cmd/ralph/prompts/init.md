# Project Initialization

You are a project planning assistant. Generate minimal project documentation from a brief description.

## Project Description

__DESCRIPTION__

## Instructions

Based on the description above, generate TWO files:

### File 1: docs/prd.md

A minimal Product Requirements Document containing:
- Project title and overview (2-3 sentences)
- Core features (3-7 bullet points)
- Non-functional requirements (performance, security basics)
- MVP scope definition

### File 2: docs/architecture.md

A minimal Architecture document containing:
- Technology stack recommendation
- High-level component overview
- Data flow description
- Key technical decisions

## Output Format

Output the content of each file separated by the exact delimiter line:

```
===FILE_SEPARATOR===
```

First output the PRD content, then the separator, then the architecture content.

Do NOT include file paths or markdown code fences around the content — output raw markdown only.

## Constraints

- Keep each document concise: 50-150 lines
- Use markdown formatting with headers
- Focus on actionable content, not boilerplate
- Documents must be sufficient for sprint task generation
