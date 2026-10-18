---
title: "MySQL"
category: "entity"
tags: [infrastructure, database, storage, schema]
source_count: 8
updated: 2026-10-18
---

# MySQL

## Overview

MySQL 8.0 is the primary data store. A single Docker container hosts 4 logically separated databases — one per service. This provides logical isolation while keeping infrastructure simple for v1. Future extraction to separate MySQL instances (or other database engines) is straightforward because schemas are already separated.

## Databases

| Database | Owner | Key Tables |
|----------|-------|------------|
| `user_db` | [[user-service]] | `users` |
| `event_db` | [[event-service]] | `events`, `event_fencing_tokens` (Ticket Service writes fencing state) |
| `ticket_db` | [[ticket-service]] | `tickets`, `outbox_events` |
| `email_db` | [[email-service]] | `email_status` |

## Schema Details

See [data-model.md](../specs/001-event-ticket-booking/data-model.md) for full entity schemas, indexes, constraints, and state machines.

### Notable Design Choices

- `user_db.users` stores PII (name, email) as `VARBINARY(512)` — encrypted at the application layer, not in MySQL. See [[pii-encryption]].
- `user_db.users.email_hash` is a SHA-256 hash — enables duplicate email detection without decrypting.
- `event_db.events.remaining_count` has a CHECK constraint: >= 0 AND <= total_capacity.
- `event_db.event_fencing_tokens` tracks the highest token issued and accepted for each event; Ticket Service uses it to fence stale writes.
- `ticket_db.tickets.booking_ref` is CHAR(12) with format `TBK-{8 alphanumeric}`, UNIQUE constraint.
- `ticket_db.outbox_events` stores the stable Kafka envelope and retry/lease metadata. Ticket Service commits it together with inventory and ticket writes using one transaction on this shared MySQL instance.
- `email_db.email_status` tracks an ENUM state machine: pending → sent | failed → dead.

## Cross-Service Data Access (v1)

All services share the same MySQL instance. Cross-database reads are done directly:

| Reader | Reads From | Purpose |
|--------|-----------|---------|
| [[ticket-service]] | `event_db.events` | Check `remaining_count` and event `date` for purchase validation |
| [[email-service]] | `user_db.users` | Fetch and decrypt email for confirmation delivery |

Tagged `TODO(v2)` — future iteration should use service APIs instead of direct cross-database reads.

## Key Decisions

- **Dedicated databases over shared schema** — provides logical isolation, makes future extraction to separate instances straightforward. See research: [Database Schema Design](../specs/001-event-ticket-booking/research.md#7-database-schema-design).
- **Single instance, multiple databases** — a pragmatic v1 choice. Separate MySQL instances per service would be more "pure" microservices but adds infrastructure overhead. Docker Compose runs one MySQL container.
- **Cross-schema booking transaction** — Ticket Service opens a transaction on the shared server and updates `event_db.events` plus `ticket_db.tickets` and `ticket_db.outbox_events`. This atomicity does not extend across separate MySQL servers.
- **No stored procedures or triggers** — all logic lives in the Go application layer. MySQL is a passive store.

## Cross-references

- [[pii-encryption]] — why encryption happens at the app layer, not in MySQL
- [[user-service]], [[event-service]], [[ticket-service]], [[email-service]] — database owners
- [[testing-strategy]] — tests run without MySQL (mock repositories, miniredis)
- [[trade-offs]] — cross-service read strategy in v1
- [[sources/config-files]] — init.sql and docker-compose.yml MySQL config
