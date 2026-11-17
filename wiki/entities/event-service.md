---
title: "Event Service"
category: "entity"
tags: [service, catalog, public, pagination]
source_count: 8
updated: 2026-11-16
---

# Event Service

## Overview

The Event Service (`:8082`) provides a public, read-only catalog of upcoming events. No authentication is required for browsing. Available inventory is calculated from the event's reservation-unit rows in MySQL.

## Responsibilities

- List upcoming events with pagination (`?page=1&per_page=20`)
- Single event detail with description, capacity, and available-unit count
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
| [[ticket-service]] | event `date` | Direct read from `event_db.events` (shared MySQL) — tagged `TODO(v2)`. Ticket Service claims units from `event_db.reservation_units` in its transaction. |

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
| `remaining_count` | INT UNSIGNED | Legacy migration value; API count is calculated from available rows in `reservation_units` |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | ON UPDATE |

Indexes: `date`; `reservation_units(event_id, status, id)` supports available-count reads and unit claims.

Full schema: [data-model.md](../specs/001-event-ticket-booking/data-model.md#event-service-database-event_db)

## Key Decisions

- **Public, no auth** — the catalog is open to unauthenticated visitors. Only ticket purchasing requires authentication.
- **Pagination without N+1** — a single query handles both count and data fetch.
- **Derived availability** — `remaining_count` is computed from available reservation units instead of the legacy event counter.
- **No filtering/search in v1** — only pagination. Search, date range, and venue filters deferred to v2 per [[trade-offs]].

## Cross-references

- [[reservation-unit-pool]] — source of the available count returned by catalog endpoints

- [[ticket-service]] — consumer of event data for purchase validation
- [[mysql]] — event_db schema
- [[trade-offs]] — deferred features (search, filtering, admin CRUD)
- [[testing-strategy]] — integration tests for event listing and detail
- [[sources/specs]] — traceability to spec requirements
- [[sources/code-structure]] — source location: `services/event/`
