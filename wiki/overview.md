---
title: "System Overview"
category: "entity"
tags: [overview, architecture, synthesis]
source_count: 11
updated: 2026-11-16
---

# System Overview

## The Big Picture

This is a distributed event ticket booking platform with four independently deployable Go services. Ticket availability is kept in MySQL reservation-unit rows and allocated transactionally, without a Redis purchase lock or a hot per-event inventory counter.

| Service | Port | Responsibility |
|---------|------|----------------|
| [[user-service]] | :8081 | Identity, authentication, PII management |
| [[event-service]] | :8082 | Public catalog of upcoming events |
| [[ticket-service]] | :8083 | Timed reservations, Stripe test checkout, paid-ticket settlement, and outbox relay |
| [[email-service]] | :8084 | Async Kafka consumer for confirmation delivery |

Infrastructure: [[mysql]] (4 databases), [[redis]] (sessions), [[kafka]] (2 topics).

## The Reservation and Payment Flow

```mermaid
sequenceDiagram
    participant Client
    participant Redis
    participant Ticket as Ticket Service
    participant MySQL
    participant Stripe
    participant CLI as Stripe CLI
    participant Kafka
    participant Email as Email Service

    Client->>Ticket: POST /api/v1/tickets/reservations
    Ticket->>Redis: auth middleware validates session
    Redis-->>Ticket: authenticated user context
    Ticket->>MySQL: BEGIN reservation transaction
    Ticket->>MySQL: SELECT available units FOR UPDATE SKIP LOCKED
    alt fewer than requested units
        Ticket->>MySQL: reclaim expired units on demand and retry
        Ticket->>MySQL: COMMIT reclamation or ROLLBACK
        Ticket-->>Client: 409 Conflict
    else enough units
        Ticket->>MySQL: create reservation; mark units reserved
        Ticket->>MySQL: COMMIT
        Ticket->>Stripe: create test PaymentIntent outside transaction
        Stripe-->>Client: client secret via Ticket Service
        Client->>Stripe: confirm card payment
        Stripe-->>CLI: signed webhook
        CLI->>Ticket: forward event
        Ticket->>MySQL: sell units; create ticket + outbox
        Ticket->>MySQL: COMMIT settlement
        Ticket->>Kafka: relay outbox event asynchronously
        Kafka--)Email: consume ticket.purchased
    end
```

## Governing Principles

All work is governed by the [[constitution]]:

1. Security-First — PII encrypted at rest.
2. Concurrency Management — claim inventory unit rows and commit the purchase atomically.
3. Service Decoupling — async messaging for side effects.
4. Test-Driven Development — unit and integration coverage for core logic.

## Key Architectural Decisions

- [[pii-encryption]] — application-level AES-256-GCM encryption
- [[reservation-unit-pool]] — MySQL unit rows spread concurrent claims and are the inventory source of truth
- [[service-decoupling]] — Kafka over direct HTTP for asynchronous side effects
- [[session-management]] — Redis-backed opaque tokens for instant revocation

## Known Limitations

- Ticket Service reads `event_db` directly → tagged `TODO(v2)` for Event Service API
- Stripe test-mode simulation only; no live payment credentials are accepted
- Expiry cleanup is on demand (when the pool is short or owner checks status), not a scheduled worker
- Single-region deployment
- [[kafka]] requires a glibc-based Docker image
- Cross-schema transaction requires one shared MySQL server; separate servers need a saga/reservation flow

See [[trade-offs]] for the full scope decisions.

## Cross-references

- [[constitution]] — governing principles
- [[reservation-unit-pool]] — inventory allocation design
- [[trade-offs]] — conscious scope decisions
- [[testing-strategy]] — test practices
- [[ci-cd-pipeline]] — CI and Docker image publishing
- [[sources/specs]] — specification traceability
- [[sources/config-files]] — infrastructure configuration
- [[sources/code-structure]] — source layout
