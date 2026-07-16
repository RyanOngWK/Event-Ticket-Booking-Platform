# Wiki Log

Chronological record of all operations: ingests, queries, lint passes, and structural changes. Append-only. Entries use a consistent date-header prefix for grep-ability.

## [2025-07-16] ingest | Initial Wiki Bootstrap

**Pages created:**
- `SCHEMA.md` — wiki conventions and workflows
- `index.md` — page catalog
- `log.md` — this file
- `entities/user-service.md`
- `entities/event-service.md`
- `entities/ticket-service.md`
- `entities/email-service.md`
- `entities/kafka.md`
- `entities/redis.md`
- `entities/mysql.md`
- `concepts/pii-encryption.md`
- `concepts/distributed-locking.md`
- `concepts/service-decoupling.md`
- `concepts/session-management.md`
- `concepts/email-retry-strategy.md`
- `concepts/testing-strategy.md`
- `concepts/constitution.md`
- `concepts/trade-offs.md`
- `overview.md`
- `sources/specs.md`
- `sources/config-files.md`
- `sources/code-structure.md`

**Sources ingested:**
- `README.md` — full project overview, architecture, API reference
- `specs/001-event-ticket-booking/spec.md` — functional requirements
- `specs/001-event-ticket-booking/plan.md` — technical architecture
- `specs/001-event-ticket-booking/research.md` — 8 design decisions with alternatives
- `specs/001-event-ticket-booking/data-model.md` — entity schemas, constraints, state machines
- `specs/001-event-ticket-booking/tasks.md` — 89 granular tasks across 7 phases
- `.specify/memory/constitution.md` — 4 governing principles v1.0.0

**Key insights captured:**
- System is a 4-service event ticket booking platform demonstrating distributed concurrency control
- Core problem: prevent double-selling under high contention (100 concurrent buyers → 1 wins)
- Constitution governs all work with 4 MUST principles; any PR violating one is rejected
- Cross-service data access is direct MySQL reads for v1; tagged TODO(v2) to migrate to service APIs
- Testing is a first-class concern: 112 test functions, zero Docker dependency at test time
- Known technical debt: Ticket Service reads event_db directly; confluent-kafka-go requires glibc-based Docker image
