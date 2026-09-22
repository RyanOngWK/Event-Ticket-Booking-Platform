---
title: "Architecture Trade-offs: Resume Deep Dive"
category: "concept"
tags: [trade-offs, architecture, technical-debt, interview-preparation, v1-scope]
source_count: 14
updated: 2026-09-22
---

# Architecture Trade-offs: Resume Deep Dive

## Overview

This is the detailed decision record for the Event Ticket Booking Platform. It is intended for a resume or system-design discussion: it distinguishes the **intentional v1 choices** from the **limitations that the current code still has**. The central design goal was to protect the inventory invariant under contention: 100 simultaneous attempts to buy the final ticket should yield one successful reservation and no negative inventory.

The useful framing is not "I used microservices, Kafka, Redis, and encryption." It is: "I optimized v1 for correctness of the purchase path, fast local iteration, and a clear path to harder production concerns. Each choice bought a specific property and introduced a specific cost."

## Executive Summary

| Decision | What it optimizes | Main cost | Production evolution |
|---|---|---|---|
| Four-service Go monorepo | Clear domain boundaries with fast iteration | Shared module and packages couple releases | Split modules/repos only when team ownership and release cadence diverge |
| One MySQL instance with four databases | Logical data ownership without four operational stacks | Shared credentials and failure domain | Separate instances, service accounts, backups, and independent scaling |
| Redis lock plus conditional SQL decrement | Low-contention purchase handling and a database safety invariant | Hot events serialize; lock lease correctness matters | Make SQL/reservations the authority; use ownership tokens or queue allocation |
| Kafka for confirmation email | Fast purchase response despite email-provider outages | DB write and event publish are not atomic | Transactional outbox or CDC |
| AES-256-GCM application encryption | Column-level PII protection without MySQL Enterprise | Key lifecycle and searchable encrypted data remain hard | KMS/Vault envelope encryption, rotation, keyed lookup hashes |
| Opaque Redis sessions | Immediate logout and centralized revocation | Redis is on every authenticated request | Add user session index and absolute lifetime; assess short-lived JWTs only if needed |
| Docker Compose and mock-heavy tests | Reproducible, fast development feedback | Limited evidence for real distributed infrastructure | Add a smaller real MySQL/Redis/Kafka CI lane |

## How To Tell The Story

Use this structure for any decision:

1. State the workload or constraint.
2. Explain the choice and the alternative deliberately not chosen.
3. Name the concrete benefit it provided in v1.
4. Name the failure mode, scaling limit, or coupling it introduced.
5. Describe the observable signal that would cause an upgrade and the next design.

Example: "Ticket inventory was the only strongly consistent business invariant, so I accepted serialization per event and used a conditional SQL decrement as the final guard. That keeps inventory non-negative even if the lock layer misbehaves. The cost is that a highly popular event becomes a hot key and the ticket write is not atomically coupled to the inventory update. At higher volume I would introduce explicit reservations and an outbox, or serialize allocations through a partitioned queue."

Do not overclaim. The specifications describe a Redlock-style lock, per-user derived encryption keys, idempotency keys, and completed dead-letter behavior. The current implementation does not fully realize those claims; the implementation notes below describe the code as it exists.

## Service Boundaries And Repository Shape

### Four bounded services in one Go module

**Choice.** User, Event, Ticket, and Email are separate deployable services. They share crypto, Kafka, and HTTP middleware under `services/shared/pkg/`, run from one Go module, and are orchestrated locally by Docker Compose. See [[user-service]], [[event-service]], [[ticket-service]], [[email-service]], and [[sources/code-structure]].

**Why it was reasonable.** The domains have different consistency and availability needs. Identity owns credentials and PII; Event owns the public catalog; Ticket owns the purchase workflow; Email owns an eventually consistent side effect. The monorepo removes cross-repository coordination and makes shared build, test, and Compose workflows simple for a four-service project.

**What it sacrifices.** Service boundaries are organizational rather than fully independent: a shared Go module and shared packages can require coordinated upgrades. There is no API gateway, discovery layer, independently versioned service SDK, or separately owned deployment repository. The operational overhead of four processes also exceeds a modular monolith at this scale.

**Alternative.** A modular monolith would be easier to debug and transact across. Separate repositories and modules would better enforce independent ownership, but slow early iteration and introduce dependency-version management.

**When to change.** Split shared modules or repositories when different teams need independent release cadence, access control, or language/runtime choice. Until then, preserve boundaries through package ownership and public contracts rather than prematurely adding delivery overhead.

### Logical database per service on one MySQL instance

**Choice.** The system has `user_db`, `event_db`, `ticket_db`, and `email_db`, all in one MySQL 8 container. The schemas establish ownership while keeping local infrastructure small. See [[mysql]] and [database initialization](../../scripts/db/init.sql).

**Benefit.** Each service has a natural data boundary and an extraction path without running four database servers. The approach avoids the coupling of a single shared schema while staying approachable for local development.

**Cost.** This is not physical isolation. The services use the MySQL root account, share CPU, storage, availability, and backup/restore fate. A noisy workload or instance failure affects all services. The schema also cannot enforce the cross-database relationships depicted conceptually with database foreign keys.

**Alternative.** Separate instances and least-privilege database accounts provide fault and security isolation, but impose more provisioning, observability, backup, and migration work. A single shared schema is simpler initially but is harder to split later.

**Interview phrasing.** "The databases establish ownership early; the shared MySQL instance is an explicit MVP operational tradeoff, not true service isolation."

### Direct cross-service database reads

**Choice.** Ticket reads and conditionally decrements `event_db.events`; Email reads and decrypts `user_db.users.email_enc`. The data model explicitly permits this for v1. See [data model](../../specs/001-event-ticket-booking/data-model.md) and [[ticket-service]].

**Benefit.** It avoids a synchronous Event or User Service network call on the critical purchase and delivery paths. It is low latency and easy to run while all schemas share one local MySQL instance.

**Cost.** It violates strict service data ownership. Ticket and Email become coupled to another service's schema, credentials, availability, encryption representation, and migration timing. It bypasses an API authorization boundary and makes independent database extraction harder. Ticket history also resolves event data per ticket, creating an N+1 lookup risk.

**Alternative.** Use Event/User APIs for fresh synchronous reads, or Kafka-fed local read models for asynchronous ownership. An inventory service or reservation projection could own availability independently.

**When to change.** Replace these reads before independently scaling or deploying services, before exposing a stronger security boundary, or when schema changes become coordination-heavy. This is the clearest documented `TODO(v2)`.

## Purchase Correctness And Concurrency

### Redis serialization plus conditional SQL inventory update

**Choice.** Ticket Service acquires a per-event Redis lock with `SET NX EX 30`, then executes a conditional update equivalent to `remaining_count = remaining_count - quantity WHERE remaining_count >= quantity`, and then inserts the ticket. See [[distributed-locking]] and `services/ticket/internal/service/purchase_service.go`.

**Benefit.** The conditional update is the important database-level invariant: no successful update can drive remaining inventory negative. The event-scoped lock reduces concurrent work and produces a prompt `423 Locked` under contention rather than allowing many requests to race through business logic. A 30-second lease bounds a stuck lock.

**Cost.** A lock per event serializes all purchasers of the same popular event, including purchasers who could otherwise buy different remaining seats. Redis adds another critical dependency. Lock acquisition fails immediately rather than queueing or retrying, shifting backoff work to clients. The lock layer controls contention but is not the sole correctness proof; the SQL predicate is.

**Alternative.** Rely on optimistic conditional updates alone; use a database row lock in a single transactional store; introduce individual seats; or partition allocation requests through a queue. Each changes the throughput, latency, complexity, and user-experience profile.

**When to change.** Monitor lock-conflict rate, purchase latency, and hot-event throughput. If the event lock becomes a bottleneck, use a reservation model or queue-based allocation rather than merely sharding a lock, because inventory remains a shared finite resource.

### The lock is a single-node lease, not Redlock

**Current implementation.** Despite older documentation calling it Redlock, `services/ticket/internal/lock/redis_lock.go` uses one Redis instance, no quorum, no acquisition retry, no fencing token, and a constant lock value of `"locked"`. v1 has no zombie-transaction protection: a request whose lease expired can keep running and act on state it no longer owns.

**Risk.** This leaves the classic zombie-transaction window open: if request A exceeds its 30-second TTL, request B can acquire the same key. A's deferred Lua release can then delete B's lock because both use the same value, and A resumes execution against state B already mutated. The conditional SQL decrement still prevents negative inventory, but mutual exclusion is no longer reliable and the system does unnecessary concurrent work.

**Fix.** Generate a cryptographically random owner token per acquisition and release only when that token matches. To close the zombie window outright, also issue a monotonic fencing token on every acquisition and require the SQL decrement to carry it: MySQL rejects any write whose fencing token is older than the last one it accepted, so a resumed zombie cannot mutate inventory even if it still holds a stale lock. For the strongest correctness story, make a transactional inventory/reservation store the authority and treat Redis only as a contention optimization.

**Interview phrasing.** "I would not describe the current code as Redlock. It is a single-Redis lease plus a SQL guard with no zombie-transaction protection. I know the distinction and would add ownership tokens plus a monotonic fencing token before production."

### Cross-database purchase atomicity

**Choice.** Availability is decremented in `event_db`; the ticket record is inserted in `ticket_db`. These occur as separate operations rather than one transaction or a saga.

**Benefit.** It respects the intended database ownership split and avoids distributed-transaction infrastructure in v1.

**Cost.** If decrement succeeds and ticket insertion fails, inventory is consumed without a ticket. Conversely, retries need an idempotency boundary to prevent accidental repeat behavior. The system has no compensation record or reservation timeout for this failure window.

**Alternative.** Co-locate inventory and ticket records in one transactional service, use a reservation state machine with expiry and compensation, or use a saga with an outbox. Two-phase commit would provide stronger atomicity but is usually avoided for availability and operational complexity.

**When to change.** This is the highest-value correctness improvement after preserving the non-negative inventory predicate. Add a purchase idempotency key and a reservation/outbox flow before handling payments or offering strong booking guarantees.

### Reservation instead of payment

**Choice.** A successful `POST /tickets/purchase` confirms a reservation-like ticket; v1 has no payment provider.

**Benefit.** The project isolates the hardest demonstrated invariant, inventory allocation, from financial workflows. It avoids PCI scope, payment authorization/capture semantics, refunds, webhooks, fraud handling, and reconciliation.

**Cost.** The word "purchase" is stronger than the behavior. A real flow needs a held reservation, payment authorization, confirmation, expiration, cancellation, and inventory release.

**Next design.** Create a `pending_payment` reservation with an expiration, publish an outbox event, authorize payment, and confirm or compensate inventory. Make payment-provider webhooks idempotent.

## Asynchronous Messaging And Email Delivery

### Kafka over synchronous service calls for side effects

**Choice.** User publishes `user.created`; Ticket publishes `ticket.purchased`; Email consumes the purchase topic and sends confirmation outside the HTTP request. See [[service-decoupling]], [[kafka-producer]], and [[kafka-consumer]].

**Benefit.** A buyer receives a booking response without waiting for Email Service or an email provider. Kafka retains messages for later consumption and permits future consumers such as analytics, audit, or notifications without changing Ticket Service.

**Cost.** The system accepts eventual consistency: email may arrive later than the booking response. It must handle duplicates, poison messages, schema evolution, consumer lag, and broker operations. `user.created` has no consumer today, which is acceptable as an extension point but has no immediate user-facing value.

**Alternative.** Direct HTTP is easier to understand but ties purchase availability to Email Service health. A job queue is lighter operationally for one worker but offers less general event-stream capability.

**Interview phrasing.** "Kafka was selected for non-blocking domain side effects, not because four services inherently require a streaming platform."

### Database write and Kafka publish are not atomic

**Current implementation.** After ticket persistence, `purchase_service.go` starts a goroutine that publishes `ticket.purchased`. The database transaction and broker publication do not share an atomic commit.

**Benefit.** The HTTP response stays fast, and waiting for a broker acknowledgement does not extend the critical path.

**Risk.** A process crash, cancellation, or publish failure after the ticket write can leave a real booking without its event and therefore without an email. Kafka `acks=all` and producer retries help only after a publish is actually attempted; they do not solve the dual-write gap.

**Next design.** Write an outbox row in the same transaction as the ticket/reservation state. A relay publishes that row with retry and marks it delivered, or use change-data capture. This converts the failure mode from silent loss into a recoverable, observable backlog.

### At-least-once consumption and idempotency

**Choice.** The shared consumer commits an offset after its handler returns success. Email checks for an existing `booking_ref` before processing. This is an at-least-once model. See [[kafka-consumer]].

**Benefit.** A crash before commit causes redelivery rather than message loss. A completed status row normally avoids a duplicate confirmation.

**Cost.** The dedupe is only a read-before-write check. `email_status.booking_ref` is indexed but not uniquely constrained, so concurrent consumers can both see no row and both send. Events lack the planned UUID idempotency key; generated event IDs are timestamp-derived. A malformed or persistently failing message can block its Kafka partition because it is not routed to a retry/DLQ topic.

**Next design.** Put a unique database constraint on the idempotency key or booking reference, claim processing with an atomic insert/upsert, use a stable UUID event ID, and add retry and dead-letter topics with alerting.

### Durable email status and retry policy

**Choice.** Email writes delivery status to `email_db`, sends through a `EmailProvider` interface, and has a scheduled retry loop using 1m, 5m, 15m, 1h, and 4h delays. The default provider writes to stdout. See [[email-retry-strategy]] and [[email-service]].

**Benefit.** Email-provider failures do not block booking. Delivery state survives restarts, and the provider interface separates business logic from SMTP, SendGrid, or another future vendor.

**Current gap.** The repository update does not persist `retry_count`; retry helpers discard their count argument. As a result, the advertised five-attempt transition to `dead` is not enforced and failed rows can retry indefinitely. The planned 24-hour maximum age is also absent. `recipient_hash` currently stores and reuses plaintext email, not a hash, and the log provider prints recipient details and message content.

**Next design.** Persist attempt count, next-attempt timestamp, last error, and terminal state atomically. Encrypt or avoid retaining recipient PII, use provider idempotency keys, move retries/DLQ to explicit workflows, and alert on age, dead-letter count, and consumer lag.

## Security And Privacy

### Application-level PII encryption

**Choice.** User name and email are encrypted before being written to MySQL using AES-256-GCM. The system stores a normalized SHA-256 email value to enforce uniqueness and look up users without decrypting every row. Passwords use bcrypt cost 12. See [[pii-encryption]] and `services/shared/pkg/crypto/crypto.go`.

**Benefit.** AES-GCM provides confidentiality and integrity at the column level, works without MySQL Enterprise TDE, and keeps the application portable across database providers. Encryption outside MySQL reduces the impact of a database-only compromise. A deterministic lookup value preserves registration/login efficiency.

**Cost.** Application encryption protects stored values, not data while it is being processed. One environment key is used directly in the current code: per-user key derivation and key versioning documented in earlier research are not implemented. An unsalted SHA-256 email hash is deterministic and susceptible to dictionary enumeration for common addresses. Encryption also complicates operational debugging, search, data export, and key rotation.

**Alternative.** MySQL TDE encrypts storage files but is coarser-grained and often edition-dependent. KMS/Vault envelope encryption offers stronger key custody and rotation but adds cloud or platform dependency. A keyed HMAC is more appropriate than raw SHA-256 for a lookup value that must resist offline guessing.

**Next design.** Store key IDs with ciphertext, use KMS/Vault envelope encryption and rotation, migrate lookups to HMAC with a separately managed key, and define re-encryption and key-loss recovery procedures.

### Opaque Redis sessions instead of JWTs

**Choice.** Login returns a random opaque token. Redis stores its user context for 24 hours and middleware refreshes the TTL on authenticated use. Logout deletes the key. See [[session-management]].

**Benefit.** Revocation is immediate and simple: delete the Redis key. Every service instance can validate sessions without sticky routing. The token contains no identity claims or sensitive data, and validation avoids JWT signature verification and blocklist complexity.

**Cost.** Redis becomes a dependency on every authenticated request; an outage impacts protected endpoints. Rolling expiry can create effectively indefinite active sessions. The middleware ignores refresh failures. Account anonymization does not currently invalidate active sessions, and there is no session index for revoking all devices after deletion or password change.

**Alternative.** Short-lived JWT access tokens reduce validation reads but still need a refresh-token/revocation design for immediate logout. Database sessions provide durability but add relational read load.

**Next design.** Add a per-user session index, fixed absolute expiry, revocation on account deletion and credential change, Redis availability monitoring, and explicit fail-open/fail-closed policy.

### Rate limiting and developer security posture

**Choice.** Registration and login use a Redis sorted-set sliding window. Docker Compose exposes all service and infrastructure ports and uses plaintext local transports and root MySQL credentials. See `services/shared/pkg/middleware/ratelimit.go` and [Docker Compose](../../docker-compose.yml).

**Benefit.** The rate limiter is a small shared protection mechanism, while the open Compose topology makes debugging and onboarding fast.

**Current gap.** The limiter keys on `RemoteAddr`, which includes the client source port, so reconnecting can bypass a per-IP limit. Its remove/count/add sequence is not atomic under concurrency, and Redis errors fail open. Compose has no TLS/mTLS, Redis authentication, Kafka ACLs, network segmentation, least-privilege database users, or secret manager. This is acceptable only as a local developer environment.

**Next design.** Trust forwarded client identity only behind a configured proxy, use an atomic Lua or gateway limiter, combine IP and account/email controls, and move production credentials/transports into managed secret, network, and identity controls.

### Deletion and privacy retention

**Choice.** Account deletion anonymizes encrypted user fields rather than deleting ticket history.

**Benefit.** It preserves booking/audit records while removing the direct user profile content, a pragmatic first step toward retention requirements.

**Cost.** Ticket records retain `user_id`; active sessions survive anonymization; no formal retention, subject-access, deletion-verification, or legal-hold workflow is implemented. It should not be described as complete PDPA compliance.

**Next design.** Define data inventory and retention schedules, revoke sessions, decide whether pseudonymous IDs qualify for each legal purpose, record erasure evidence, and obtain legal/privacy review.

## API, Data, And Product Scope

### Minimal public API and no gateway

**Choice.** Each HTTP service exposes its own port with `net/http` and `gorilla/mux`; there is no frontend, BFF, gateway, service discovery, or service mesh. Event browsing supports pagination but not search or filters.

**Benefit.** The HTTP surface is small, direct, and easy to test. Pagination validates catalog behavior without inventing unproven search requirements. Avoiding a gateway removes another availability and configuration layer.

**Cost.** Clients must know multiple service endpoints. Cross-cutting concerns are duplicated in service wiring. There is no centralized authentication edge, request shaping, API version lifecycle, or public traffic policy. Pagination alone will not meet discovery needs once catalog volume or product requirements grow.

**Next design.** Add a gateway/BFF only when clients, public exposure, or cross-cutting policies justify it. Add search/filtering from product evidence, likely backed by indexes or a search read model rather than arbitrary database queries.

### Seeded catalog and no administration workflow

**Choice.** Events are loaded with seed data; there is no admin event CRUD interface.

**Benefit.** It keeps the project focused on read catalog behavior and inventory correctness. Administration is a separate authorization, audit, and product domain.

**Cost.** The product cannot be operated by a real event organizer. There are no roles, audit logs, event publishing lifecycle, inventory adjustments, or content moderation controls.

**Next design.** Introduce an administrative bounded context with RBAC, auditability, validation, and a clear policy for changing inventory after tickets exist.

### Single region

**Choice.** The topology is one region with one Redis and one Kafka broker.

**Benefit.** The concurrency model is understandable: all purchasers contend on the same Redis and write to the same MySQL instance. It avoids cross-region replication and clock/failover complexity while validating the core flow.

**Cost.** A regional outage is a total outage. Cross-region locks, replicated inventory, Kafka replication, session locality, latency, and split-brain concerns are unresolved.

**Next design.** Start with managed multi-AZ data services in one region. Expand to multi-region only for a quantified availability/latency requirement, and make one region or partition authoritative for a given event's inventory.

## Operations, Deployment, And Delivery

### Docker Compose over Kubernetes

**Choice.** Docker Compose runs MySQL, Redis, ZooKeeper, Kafka, topic initialization, and four services. The Dockerfile is a reusable, CGO-enabled Debian build for all services. See [[sources/config-files]].

**Benefit.** One command starts the full topology and health dependencies. A single parameterized Dockerfile reduces duplicated build logic. Debian avoids Alpine `musl` compatibility issues with the native `confluent-kafka-go` dependency.

**Cost.** Compose is a local orchestration tool, not a production scheduler. The Kafka configuration is single-broker with replication factor one and uses ZooKeeper-era setup. There are no resource requests/limits, autoscaling, backups, immutable image digests, dependency-aware readiness checks, or disaster-recovery plan. Debian images are generally larger than Alpine.

**Alternative.** Kubernetes, ECS, Nomad, or a managed container platform add scheduling, rollout, and scaling capabilities but impose operational and local-development cost. Managed Kafka/MySQL/Redis shift reliability work to a provider.

**Interview phrasing.** "Compose optimizes developer experience. Production readiness means managed dependencies, immutable artifacts, resource policies, real readiness, backup/restore, and deployment automation, not simply converting Compose YAML to Kubernetes YAML."

### CI builds and tests, but does not deploy

**Choice.** GitHub Actions builds/vets Go, runs test jobs, builds four images in parallel, and pushes images to GHCR on `main`. See [[ci-cd-pipeline]].

**Benefit.** Every change has repeatable build/test evidence. Per-service image builds are parallelized, and commit SHA tags give a more traceable artifact than `latest` alone.

**Cost.** The pipeline has no deployment, environment promotion, smoke test against a deployed stack, coverage gate, schema migration gate, image/dependency scan, SBOM, signature, or vulnerability check. `latest` remains mutable. CI confidence is bounded by the mock-heavy test design below.

**Next design.** Produce immutable signed images and SBOMs, add dependency/image scanning and coverage reporting, deploy a staging environment from an immutable SHA, run smoke/contract tests, and promote with rollback criteria.

### Observability is logging-first

**Choice.** Shared middleware provides structured request logs and correlation IDs; metrics, tracing backend, dashboards, and alerts are deferred.

**Benefit.** Correlation IDs give a useful manual trail across HTTP and event-related logs with minimal infrastructure.

**Cost.** Logs alone do not answer whether inventory contention, Kafka lag, email failure, Redis availability, or latency SLOs are degrading. Debugging a distributed failure remains expensive, especially when asynchronous event propagation is involved. PII must be carefully excluded from every log path; the current log email provider violates that boundary.

**Next design.** Add RED metrics per API, business metrics such as successful reservations and sold-out/lock-conflict rates, Kafka consumer lag, email age/dead-letter count, distributed traces with propagated correlation IDs, dashboards, and SLO-driven alerts.

## Testing Strategy And Its Limits

### Fast, deterministic tests over real infrastructure

**Choice.** The suite uses Go's test tooling, `httptest`, mocks/stubs, and `miniredis`. Tests are designed to run without Docker, MySQL, or Kafka for rapid feedback. See [[testing-strategy]].

**Benefit.** Developers can run a fast suite locally and in CI without environment flakiness. Unit tests cover crypto, validation, authentication, pagination, handlers, and lock behavior. A concurrency test exercises the high-level inventory scenario.

**Cost.** Most "integration" tests use mocked repositories or embedded Redis rather than the actual MySQL schema and Kafka broker. The E2E test builds an in-process router with stubs and covers registration/login/browse, not a real four-service purchase-to-email flow. The MySQL purchase integration test skips if local MySQL is absent. This does not validate Kafka producer acknowledgements, consumer loop commit behavior, schema drift, cross-database partial failures, lock TTL expiry, Redis outage behavior, or actual email retry persistence.

**Alternative.** Testcontainers or a Compose CI job provide higher fidelity but are slower, more expensive, and more failure-prone. Contract tests improve service/API compatibility without requiring the complete stack.

**Next design.** Keep the fast suite, then add a small mandatory infrastructure lane for the riskiest invariants: real MySQL migrations, Redis lease ownership/expiry, Kafka produce-consume and duplicate delivery, transactional outbox recovery, and a cross-service reservation/email smoke test. Run `go test -race` for concurrent paths.

**Accurate claim.** Avoid saying every integration test is infrastructure-backed or that the E2E suite covers purchase/email. Count test functions from the repository at the time of a presentation rather than relying on stale documentation.

## Deferred Product And Platform Scope

| Area | v1 decision | Why it was deferred | What it would require |
|---|---|---|---|
| Payment | Booking confirms without payment | Keeps PCI and financial state out of the inventory proof | Reservation expiry, provider integration, webhook idempotency, refunds, reconciliation |
| MFA and social login | Email/password only | Orthogonal to purchase correctness | Identity-provider integration, recovery flows, risk controls |
| Search/filtering | Pagination only | No evidence yet for search relevance needs | Query/index design or search read model, product relevance rules |
| Admin event management | Seed data only | Separate operator domain | RBAC, audits, lifecycle, inventory-change policy |
| Multi-region | Single region | Distributed inventory coordination is complex | Authoritative partitioning, replication/failover strategy, regional SLOs |
| Kubernetes | Compose only | No deployment-scale requirement in v1 | Platform operations, secrets, policies, rollout/observability practices |
| Gateway/gRPC | Direct public service endpoints and database reads | Avoids adding network layers before need | Versioned service contracts, auth/policy edge, service discovery or mesh |
| Real email provider | Stdout provider | Allows deterministic local verification | Credentials, provider idempotency, deliverability, bounce handling |
| Full monitoring | Logs and correlation IDs | Reduces MVP infrastructure | Metrics, tracing, dashboards, alerts, on-call/runbooks |

## Resume Deep-Dive Checklist

- Lead with the inventory invariant, then explain why SQL is the final non-negative guard.
- Describe Redis as a contention-control optimization with a bounded lease, not as a complete distributed-consensus solution.
- Explain that Kafka separates booking availability from email availability, then volunteer that an outbox is required for atomic delivery guarantees.
- Present database-per-service as logical ownership on shared infrastructure, not full isolation.
- Explain application encryption's control and portability, then identify key rotation and deterministic lookup hashes as production hardening work.
- State that opaque sessions intentionally traded statelessness for immediate revocation.
- State that fast mocked tests optimize developer feedback, and name the real-infrastructure scenarios still needed.
- Separate consciously deferred scope from implementation bugs or incomplete behavior. Knowing that distinction demonstrates engineering judgment.

## Cross-references

- [[overview]] - system topology and critical purchase flow
- [[constitution]] - non-negotiable security, concurrency, decoupling, and TDD principles
- [[distributed-locking]] - Redis lock design and implementation detail
- [[pii-encryption]] - encrypted PII storage
- [[session-management]] - opaque session behavior
- [[service-decoupling]] - event-driven side effects
- [[kafka-producer]] and [[kafka-consumer]] - producer and consumer implementation walkthroughs
- [[email-retry-strategy]] - email delivery state and retries
- [[testing-strategy]] - test pyramid and test-environment constraints
- [[ci-cd-pipeline]] - CI and image publication
- [[sources/specs]] and [[sources/config-files]] - raw decision and infrastructure sources
