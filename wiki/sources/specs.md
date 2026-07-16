---
title: "Source Map: Specs"
category: "source-map"
tags: [sources, spec, requirements, research]
source_count: 3
updated: 2025-07-16
---

# Source Map: Specification Documents

## Overview

The specification documents in `specs/001-event-ticket-booking/` form the design blueprint. Every line of code traces back to a requirement in these files. The full decision trail is preserved: constitution → spec → plan → tasks → implementation.

## Files

| File | Description | Maps to Wiki Pages |
|------|-------------|-------------------|
| `spec.md` | 20 functional requirements, 8 success criteria, 3 user stories, edge cases | [[overview]], all entity pages, [[constitution]] |
| `plan.md` | Tech stack decisions, service decomposition, file layout | [[overview]], all entity pages |
| `tasks.md` | 89 granular tasks across 7 phases, each with file paths and dependencies | [[sources/code-structure]] |
| `research.md` | 8 technical decisions with rationale and rejected alternatives | [[pii-encryption]], [[distributed-locking]], [[service-decoupling]], [[session-management]], [[email-retry-strategy]], [[mysql]], [[kafka]] |
| `data-model.md` | Entity schemas, indexes, constraints, state transitions | [[user-service]], [[event-service]], [[ticket-service]], [[email-service]], [[mysql]] |
| `contracts/` | REST API specs and Kafka topic schemas | All entity pages (API tables), [[kafka]] |
| `quickstart.md` | Runnable validation scenarios | [[testing-strategy]] e2e test |
| `checklists/` | Spec quality validation checklist | [[testing-strategy]] |

## Ingest History

- **2025-07-16**: Full bootstrap — all spec files ingested to create initial wiki pages. See [[log|log.md#2025-07-16-ingest-initial-wiki-bootstrap]].
