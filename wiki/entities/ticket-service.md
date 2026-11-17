---
title: "Ticket Service"
category: "entity"
tags: [service, purchase, concurrency, inventory]
source_count: 9
updated: 2026-11-16
---

# Ticket Service

## Overview

The Ticket Service (`:8083`) owns authenticated timed reservations, Stripe test-mode checkout, paid tickets, and purchase history. It claims individual MySQL reservation-unit rows, creates a PaymentIntent outside the database transaction, and settles payment only after a verified Stripe webhook.

## Responsibilities

- Claim available inventory units without a Redis purchase lock
- Create time-limited reservations and return a PaymentIntent client secret
- Lazily reclaim expired reservations when an event's available unit pool cannot satisfy a request
- Refund a successful payment if its reservation expired before settlement
- Commit sold unit rows, ticket, reservation status, and outbox event in one MySQL transaction after payment
- Generate unique booking references (`TBK-{8 alphanumeric}`)
- Relay committed `ticket.purchased` outbox records to [[kafka]] with retry
- Retrieve paginated purchase history with event details
- Run a local 10,000-client reservation burst using the real handler/service with isolated event data and a fake payment gateway (`make simulate-reservations`)

## Interfaces

### REST API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/checkout` | — | Local Stripe test-mode checkout page |
| `POST` | `/api/v1/tickets/reservations` | Bearer | Hold units and create a Stripe PaymentIntent. Body: `{"event_id": 1, "quantity": 2}` |
| `GET` | `/api/v1/tickets/reservations/{id}` | Bearer | Get own reservation state and booking reference |
| `POST` | `/api/v1/payments/stripe/webhook` | Stripe signature | Verify and process test-mode payment events |
| `GET` | `/api/v1/tickets` | Bearer | Purchase history with nested event details |

### HTTP Status Codes (Reservation)

| Code | Meaning |
|------|---------|
| `201` | Reservation created; ticket confirmation follows webhook processing |
| `400` | Invalid input or event has passed |
| `401` | Missing/expired session |
| `404` | Event or reservation not found |
| `409` | Sold out or fewer than requested units available |

### Kafka

After a successful payment webhook, the service writes the stable `ticket.purchased` event envelope, including `event_id` and `correlation_id`, to the outbox in the same transaction that creates the ticket and sells units. A background relay publishes it to Kafka.

## Purchase Flow

```mermaid
sequenceDiagram
    participant Client
    participant Redis
    participant Ticket as Ticket Service
    participant MySQL
    participant Stripe
    participant CLI as Stripe CLI
    participant Relay as Outbox Relay
    participant Kafka

    Client->>Ticket: POST /api/v1/tickets/reservations
    Ticket->>Redis: auth middleware GET session:{token}
    Redis-->>Ticket: user context (user_id)
    Ticket->>MySQL: BEGIN reservation transaction
    Ticket->>MySQL: SELECT available unit rows FOR UPDATE SKIP LOCKED
    alt fewer than requested units found
        Ticket->>MySQL: reclaim expired reservations if pool is short; retry claim
        Ticket->>MySQL: COMMIT reclamation or ROLLBACK
        Ticket-->>Client: 409 Conflict
    else enough units found
        Ticket->>MySQL: create pending reservation; mark units reserved
        Ticket->>MySQL: COMMIT reservation
        Ticket->>Stripe: create PaymentIntent outside SQL transaction
        Stripe-->>Client: client secret (through Ticket Service)
        Client->>Stripe: confirm card with Payment Element
        Stripe-->>CLI: signed payment event
        CLI->>Ticket: forward webhook
        Ticket->>MySQL: verify reservation; sell units; create ticket + outbox
        Ticket->>MySQL: COMMIT settlement
        Relay->>Kafka: publish outbox event and retry on failure
    end
```

Redis in the auth step is only for session validation; it does not coordinate inventory.

## Data Model

`event_db.reservation_units` is the inventory authority. Each row represents one saleable unit and has `available`, `reserved`, or `sold` status, an optional `reservation_id`, and an optional `ticket_id`. `ticket_db.reservations` tracks owner, amount, Stripe PaymentIntent, state, and expiry. `ticket_db.tickets` stores the aggregate purchased quantity; `ticket_db.outbox_events` stores the durable event envelope.

Full schema: [data-model.md](../specs/001-event-ticket-booking/data-model.md)

## Key Decisions

- **Reservation-unit pool** — unit rows spread concurrent claims across inventory rows. `FOR UPDATE SKIP LOCKED` prevents two transactions from claiming the same unit. See [[reservation-unit-pool]].
- **Payment outside SQL transaction** — the reservation and unit hold commit before calling Stripe, so network latency does not hold inventory row locks.
- **Webhook settlement transaction** — unit sale, ticket insertion, reservation completion, and outbox insertion commit together after signed webhook verification.
- **Lazy expiry reclaim** — when the pool cannot satisfy a request, pending expired reservations are marked expired and their units are released in a bounded transaction. A late success webhook is refunded idempotently.
- **Derived availability** — Event Service calculates `remaining_count` from available unit rows; the old counter is retained only for existing schema compatibility.
- **Transactional outbox** — Kafka publication is asynchronous and at least once. Consumers remain idempotent.

## Known Technical Debt

- **Shared-instance transaction boundary**: inventory is in `event_db`; tickets and outbox are in `ticket_db`. Atomicity depends on both schemas sharing one MySQL server. Separate servers require a saga/reservation workflow.
- **Cross-database read**: Ticket Service reads `event_db.events` directly for validation. Tagged `TODO(v2)` — future iteration should use Event Service API calls.
- **Retry on skipped locks**: `SKIP LOCKED` may find too few rows while other transactions hold units, so a safe request can receive a conflict and retry.
- **No scheduled expiry worker**: expired reservations are reclaimed on demand when more inventory is needed, or when their owner checks status. Idle expired rows can remain marked pending until one of those operations occurs.
- **Stripe is simulated in test mode**: no live payment credentials are accepted; Stripe CLI forwards signed webhooks locally.

## Cross-references

- [[reservation-unit-pool]] — inventory concurrency approach
- [[mysql]] — reservation and settlement tables
- [[redis]] — session validation only
- [[kafka]] — `ticket.purchased` topic
- [[kafka-producer]] — `TicketEventPublisher` walkthrough
- [[email-service]] — consumer of purchase events
- [[event-service]] — event details and available inventory count
- [[mysql]] — cross-database booking transaction
- [[constitution]] — Principle II (Concurrency Management)
- [[trade-offs]] — purchase architecture decisions
- [[sources/code-structure]] — source location: `services/ticket/`
