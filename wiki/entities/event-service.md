---
title: "Event Service"
category: "entity"
tags: [service, catalog, public, pagination]
source_count: 7
updated: 2025-07-16
---

# Event Service

## Overview

The Event Service (`:8082`) provides a public, read-only catalog of upcoming events. No authentication required for browsing. Supports pagination without N+1 queries.

## Responsibilities

- List upcoming events with pagination (`?page=1&per_page=20`)
- Single event detail with description, capacity, remaining count
- Sold-out indicator in API responses
- Database seeding (6 sample events via `make seed`)

## Interfaces

### REST API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/events` | — | List events. Query: `?page=1&per_page=20`. Returns nested `pagination` object. |
| `GET` | `/api/v1/events/{id}` | — | Event detail with `remaining_count` |

### Internal (consumed by other services)

| Consumer | Data Needed | Access Method |
|----------|-------------|---------------|
| [[ticket-service]] | `remaining_count`, event `date` | Direct read from `event_db.events` (shared MySQL) — tagged `TODO(v2)` |

## Data Model

Database: `event_db`. Table: `events`.

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT |
| `name` | VARCHAR(255) | |
| `description` | TEXT | |
| `date` | DATETIME | Must be in the future for active events |
| `venue` | VARCHAR(255) | |
| `total_capacity` | INT UNSIGNED | > 0 |
| `remaining_count` | INT UNSIGNED | CHECK >= 0, <= total_capacity |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | ON UPDATE |

Indexes: `date`, `remaining_count` (for sold-out filtering).

Full schema: [data-model.md](../specs/001-event-ticket-booking/data-model.md#event-service-database-event_db)

## Key Decisions

- **Public, no auth** — the catalog is open to unauthenticated visitors. Only ticket purchasing requires authentication.
- **Pagination without N+1** — a single query handles both count and data fetch.
- **No filtering/search in v1** — only pagination. Search, date range, and venue filters deferred to v2 per [[trade-offs]].

## Cross-references

- [[ticket-service]] — consumer of event data for purchase validation
- [[mysql]] — event_db schema
- [[trade-offs]] — deferred features (search, filtering, admin CRUD)
- [[testing-strategy]] — integration tests for event listing and detail
- [[sources/specs]] — traceability to spec requirements
- [[sources/code-structure]] — source location: `services/event/`
