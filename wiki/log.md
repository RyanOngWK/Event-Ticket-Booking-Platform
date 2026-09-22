# Wiki Log

Chronological record of all operations: ingests, queries, lint passes, and structural changes. Append-only. Entries use a consistent date-header prefix for grep-ability.

## [2026-09-22] query | Zombie Transaction Protection in Trade-offs

**Prompt:** Did trade-offs mention the lack of zombie-transaction protection in v1? Include a monotonic fencing token in the future implementation.

**Pages updated:**
- `concepts/trade-offs.md` - the single-node lease section now explicitly names the missing v1 zombie-transaction protection, describes the zombie window (stale lock holder releasing B's lock and resuming against mutated state), and prescribes a monotonic fencing token enforced by the SQL decrement as the fix.

**Key insights:**
- Previously the page mentioned "no fencing token" but never named the zombie-transaction failure mode or listed it as absent v1 protection.
- Fencing token enforcement lives in the storage layer (conditional decrement rejects stale tokens), not in Redis alone.

## [2026-09-02] query | Architecture Trade-offs Resume Deep Dive

**Prompt:** Create a detailed markdown record of the project's trade-offs for a resume deep dive.

**Pages updated:**
- `concepts/trade-offs.md` - replaced the high-level future-scope list with an evidence-based decision record covering service boundaries, data ownership, concurrency, messaging, security, product scope, operations, and tests.
- `index.md` - updated the trade-offs page summary.

**Key insights:**
- The detailed page distinguishes intentional v1 scope from current implementation gaps, including the single-node Redis lease, non-atomic database/event dual write, retry-state persistence, direct cross-service database reads, and mock-heavy integration coverage.
- It includes interview framing and concrete production-evolution triggers so project claims remain accurate and defensible.

## [2026-08-12] query | Kafka Producer & Consumer Code Walkthroughs

**Prompt:** User (Go beginner) asked to understand the Kafka producer and consumer code — what each file means.

**Pages created:**
- `concepts/kafka-producer.md` — code walkthrough of `services/shared/pkg/kafka/producer.go` and the per-service publishers (ticket, user). Covers `EventEnvelope` wire format, `Publish` flow (envelope → marshal → produce → `<-deliveryChan` ack), `acks=all`/`retries=5` rationale, and the untested-producer gap.
- `concepts/kafka-consumer.md` — code walkthrough of `services/shared/pkg/kafka/consumer.go` (Consume loop, register-by-topic, commit-after-success → at-least-once) and `EmailConsumer.HandleTicketPurchased` (dedupe, email fetch/decrypt, pending→sent/failed status). Covers the `userEmailFunc` test seam and how tests bypass the broker.

**Pages updated:**
- `index.md` — added both pages to Concepts table
- Cross-references added on: `entities/kafka.md`, `entities/email-service.md`, `entities/ticket-service.md`, `entities/user-service.md`, `concepts/service-decoupling.md`, `sources/code-structure.md`

**Key insights:**
- Producer `Publish` is synchronous — blocks on broker ack via channel, so it's untestable without a real broker and the publishers hold a concrete `*kafka.Producer` (no mock seam)
- Consumer achieves at-least-once via commit-after-success; handler must be idempotent → dedupe on `booking_ref`
- Handler returns `nil` on send failure intentionally — records `failed` status for retry rather than blocking the loop
- Existing consumer tests call the handler directly with a fabricated `kafkago.Message`, bypassing the shared loop (fidelity gap)

## [2025-07-17] ingest | CI/CD Pipeline — Docker Build & Push

**Pages created:**
- `concepts/ci-cd-pipeline.md` — GitHub Actions CI pipeline overview, docker build+push flow, registry layout

**Pages updated:**
- `index.md` — added ci-cd-pipeline to concepts section
- `sources/config-files.md` — added `.github/workflows/ci.yml` entry, updated docker-compose and Dockerfile descriptions to reference CI, bumped source_count
- `concepts/trade-offs.md` — added Docker build+push to "Design Decisions That Will Scale"

**Sources ingested:**
- `.github/workflows/ci.yml` — 5-job pipeline (build-and-vet, unit, integration, e2e, docker)
- `docker-compose.yml` — updated with GHCR image references

**Key insights:**
- CI now builds and pushes all 4 service images to GHCR on main push; PRs build-only
- Matrix strategy runs 4 parallel docker builds, one per service
- GHCR auth uses built-in GITHUB_TOKEN — no external secrets
- `docker-compose.yml` has both `image:` and `build:` blocks — local dev still builds locally, `docker compose pull` fetches from GHCR
- Workflow concurrency group cancels in-progress runs on rapid pushes

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
