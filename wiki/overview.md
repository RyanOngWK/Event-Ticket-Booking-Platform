---
title: "System Overview"
category: "entity"
tags: [overview, architecture, synthesis]
source_count: 9
updated: 2026-10-18
---

# System Overview

## The Big Picture

This is a **distributed event ticket booking platform** — 4 independently deployable Go microservices that demonstrate production-grade concurrency control. The core problem: prevent double-selling tickets under high contention.

Four services, each owning a single bounded context:

| Service | Port | Responsibility |
|---------|------|----------------|
| [[user-service]] | :8081 | Identity, authentication, PII management |
| [[event-service]] | :8082 | Public catalog of upcoming events (read-only for visitors) |
| [[ticket-service]] | :8083 | Redlock/fencing purchase flow with transactional outbox relay |
| [[email-service]] | :8084 | Async Kafka consumer for confirmation delivery |

Infrastructure: [[mysql]] (4 databases), [[redis]] (sessions + locks), [[kafka]] (2 topics).

## The Purchase Flow (Critical Path)

```mermaid
sequenceDiagram
    participant Client
    participant Ticket as Ticket Service
    participant Redis
    participant MySQL
    participant Kafka
    participant Email as Email Service

    Client->>Ticket: POST /api/v1/tickets/purchase
    Ticket->>Redis: GET session:{token}
    Redis-->>Ticket: user context

    Ticket->>MySQL: allocate monotonic fencing token
    Ticket->>Redis: acquire 5-master Redlock lease (30s)
    alt lock failed
        Ticket-->>Client: 423 Locked
    end

    Ticket->>MySQL: conditional inventory decrement + accept fencing token
    alt zero rows
        Ticket->>Redis: RELEASE lock
        Ticket-->>Client: 409 Sold out
    else success
        Ticket->>MySQL: INSERT INTO tickets (...)
        Ticket->>MySQL: commit inventory + ticket + outbox
        Relay->>Kafka: publish outbox event and retry on failure
        Ticket->>Redis: RELEASE lock
        Ticket-->>Client: 201 Created

        Kafka--)Email: consume
        Email->>Email: check idempotency
        Email->>MySQL: decrypt email from user_db
        Email->>Email: send + retry with backoff
    end
```

## Governing Principles

All work is governed by the [[constitution]] — 4 MUST principles:
1. **Security-First** — PII encrypted at rest (AES-256-GCM)
2. **Concurrency Management** — Distributed locking for all purchases
3. **Service Decoupling** — Async messaging for all side effects
4. **Test-Driven Development** — 80% minimum coverage on business logic

Violating any principle = PR rejected.

## Key Architectural Decisions

- [[pii-encryption]] — Application-level AES-256-GCM over MySQL TDE (column-level control, DB-agnostic)
- [[distributed-locking]] — five-master Redlock lease for contention plus MySQL fencing at the write boundary
- [[service-decoupling]] — Kafka over direct HTTP between services (decouples availability)
- [[session-management]] — Redis-backed opaque tokens over JWTs (instant revocation without blocklist)

## Known Limitations (v1)

- [[ticket-service]] reads `event_db` directly → tagged `TODO(v2)` for Event Service API
- No payment processing — purchase = reservation only
- Single-region deployment only
- [[kafka]] dependency requires glibc-based Docker image (Debian, not Alpine)
- Cross-service reads use shared MySQL; gRPC between services deferred

See [[trade-offs]] for the full v1 scope decisions.

## Testing

[[testing-strategy]]: 112 test functions (87 unit + 24 integration + 1 e2e). Zero Docker dependency. Full suite completes in <15 seconds.

## Development Pipeline

The project was built using a spec-driven AI pipeline: Constitution → Spec → Plan → Tasks → Implement → Converge. Every line of code traces back to a requirement.

## CI/CD

[[ci-cd-pipeline]]: GitHub Actions runs on every push and PR — build/vet, unit, integration, e2e tests. On main push, all 4 service images are built and pushed to `ghcr.io/ryanongwk/event-ticket-booking-platform`. `docker-compose.yml` pulls pre-built images or builds locally.

## Cross-references

- [[constitution]] — governing principles
- [[trade-offs]] — conscious v1 decisions and deferred scope
- [[testing-strategy]] — test pyramid and practices
- [[ci-cd-pipeline]] — GitHub Actions workflow, Docker image build+push
- [[sources/specs]] — traceability from wiki to raw specification documents
- [[sources/config-files]] — infrastructure and build configuration
- [[sources/code-structure]] — source code layout
