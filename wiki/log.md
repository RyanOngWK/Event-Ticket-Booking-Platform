# Wiki Log

## [2026-11-16] simulation | 10,000-Client Reservation Burst

**Prompt:** Simulate 10,000 fake concurrent users against the real reservation path.

**Implementation:**
- Added `services/ticket/cmd/loadsim` and `make simulate-reservations`.
- The runner uses the production reservation handler/service, Redis auth middleware, real MySQL unit allocation, a temporary event, synthetic Redis sessions, and a fake payment gateway. Stripe is not called. Temporary data is removed after the run.
- The burst released 10,000 requests together, with up to 1,000 HTTP connections and a 100-connection MySQL pool.

**Result:**
- 10,000 requests completed in 9.429 seconds (about 1,060.6 offered requests/second).
- 500 returned `201`; 9,500 returned `409`; zero returned `5xx` or transport errors.
- 500 pending reservations matched 500 reserved unit rows for a 500-unit event; no oversell occurred.
- Request latency was p50 5.316s, p95 9.088s, and p99 9.352s. These include client-side waiting under the 1,000-connection cap.
- Verified cleanup: no temporary load event remained and no `session:loadsim-*` Redis keys remained.

**Pages updated:**
- `concepts/testing-strategy.md`, `entities/ticket-service.md`, `index.md`, `sources/code-structure.md`, and `sources/config-files.md`.

## [2026-11-16] update | Stripe Test-Mode Checkout Simulation

**Prompt:** Implement an end-to-end simulation of external payment confirmation and webhook settlement.

**Pages updated:**
- `entities/ticket-service.md` and `concepts/reservation-unit-pool.md` - document timed reservations, Stripe PaymentIntent setup, signed webhook settlement, on-demand expiration reclaim, and late-success refunds.
- `entities/mysql.md` and `overview.md` - map the reservation table and the updated payment sequence.
- `concepts/trade-offs.md` - replace the stale no-payment scope statement with test-mode simulation and production gaps.
- `index.md`, `sources/config-files.md`, and `sources/code-structure.md` - refresh summaries and source mappings.

**Key insights:**
- External payment happens after a short reservation commit, keeping Stripe latency outside inventory row locks.
- The verified success webhook commits sold units, ticket, reservation status, and outbox event atomically on the shared MySQL instance.
- Expiry reclamation runs on demand; a successful payment arriving after expiry triggers an idempotent test-mode refund.

## [2026-11-16] update | Flash Sale Reservation-Unit Pool

**Prompt:** Use the attached flash-sale design as the source of truth and replace Redis purchase concurrency checks with a per-product reservation-unit pool.

**Pages created/updated:**
- Created `concepts/reservation-unit-pool.md` with the source design, implementation mapping, and scope boundary for this synchronous purchase API.
- Updated `entities/ticket-service.md`, `entities/event-service.md`, `entities/mysql.md`, `entities/redis.md`, `overview.md`, `concepts/constitution.md`, `concepts/trade-offs.md`, and `concepts/testing-strategy.md`.
- Updated `index.md`, `SCHEMA.md`, `sources/config-files.md`, `sources/code-structure.md`, and `sources/specs.md`.

**Key insights:**
- One MySQL row represents one saleable unit. `FOR UPDATE SKIP LOCKED` lets concurrent purchase transactions claim different rows without a separate Redis inventory authority.
- Temporary payment reservations, expiry reclamation, and a waiting room are described by the PDF but remain outside this project's immediate-confirmation purchase API.
- Redis remains for session/authentication state.

## [2026-10-18] update | Transactional Booking and Kafka Outbox

**Prompt:** Implement stage three: make inventory decrement, ticket creation, and Kafka event intent atomic.

**Pages updated:**
- `entities/ticket-service.md`, `entities/mysql.md`, and `overview.md` - document the shared-MySQL transaction and asynchronous outbox relay.
- `concepts/trade-offs.md` - replace the cross-database partial-write gap with the current shared-instance boundary and at-least-once delivery trade-off.
- `index.md` and `sources/code-structure.md`, `sources/config-files.md` - refresh summaries and source mappings.

**Key insights:**
- Inventory, ticket, and the stable event envelope now commit in one transaction across `event_db` and `ticket_db` on the shared MySQL server.
- The relay claims rows with a lease, retries failures with backoff, and marks successful delivery. A crash after Kafka acknowledgment may cause a duplicate; delivery is at least once.
- Moving the schemas to separate database servers requires a saga/reservation flow or a new ownership boundary; this local transaction cannot span servers.

## [2026-10-18] update | Ticket Purchase Fencing Token

**Prompt:** Implement stage two of the purchase-correctness plan by adding a fencing token after Redlock.

**Pages updated:**
- concepts/distributed-locking.md - documents five-master Redlock, pre-acquisition MySQL token allocation, acceptance checks, and remaining atomicity limits.
- entities/ticket-service.md, entities/redis.md, entities/mysql.md, and overview.md - describe the implemented lock and fencing flow.
- concepts/trade-offs.md and index.md - replace stale single-node/no-fencing claims.
- sources/specs.md, sources/config-files.md, and sources/code-structure.md - map the data-model addition, migration, Redis nodes, and Ticket Service implementation.

**Key insights:**
- MySQL issues monotonic per-event tokens before Redlock acquisition; failed lock attempts may leave gaps.
- The inventory update stores the highest accepted token and rejects an older token after a newer one has been accepted.
- Inventory decrement and ticket insertion are still separate; stage three remains a transactional booking plus outbox.


Chronological record of all operations: ingests, queries, lint passes, and structural changes. Append-only. Entries use a consistent date-header prefix for grep-ability.

## [2026-09-22] update | README Trade-offs — Zombie Transaction Note

**Prompt:** Add the zombie-transaction/fencing-token gap to the README's Trade-offs & Future Scope section (explicit user request; README is otherwise raw source).

**Pages updated:**
- `README.md` - deferred-scope table gained a "Zombie transaction protection" row; Known Technical Debt gained a bullet describing the constant lock value, stale-lease release risk, and the owner-token + monotonic fencing-token fix.

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
## [2026-11-16] query | Flash Sale Interview Trade-offs

**Prompt:** Save the detailed discussion of interview tradeoffs for the flash-sale design and current implementation.

**Pages created/updated:**
- Created `concepts/interview-tradeoffs.md` with interview-ready explanations of inventory authority, reservation holds, Stripe settlement, expiry reclamation, waiting-room scope, outbox atomicity, data-model costs, failure cases, and load-test limits.
- Updated `index.md` and `concepts/reservation-unit-pool.md` with links to the guide.

**Key clarifications:**
- Stripe latency does not hold MySQL row locks; the committed reservation holds inventory state for a bounded checkout period.
- On-demand reclamation avoids relying on cron timing for purchase-path availability, while lazy cleanup and the 100-reservation batch limit remain tradeoffs.
- The recorded 10,000-request local simulation is not evidence of Stripe or million-user production capacity.

## [2026-11-16] update | Interview Guide Architecture Diagram

**Prompt:** Add a high-level system architecture to the beginning of the interview tradeoffs Markdown guide.

**Pages updated:**
- `concepts/interview-tradeoffs.md` - added a Mermaid architecture diagram and the main purchase-flow steps, including Stripe webhook settlement, outbox publishing, Redis session scope, and on-demand expiry reclamation.
