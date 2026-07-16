---
title: "Service Decoupling"
category: "concept"
tags: [architecture, kafka, async, messaging, coupling]
source_count: 7
updated: 2025-07-16
---

# Service Decoupling

## Overview

All non-blocking operations use async messaging via [[kafka]] rather than synchronous HTTP calls between services. This is a MUST principle under the [[constitution]] (Principle III: Service Decoupling). The rule: synchronous HTTP is permitted only for operations requiring immediate consistency.

## Rationale

**Why Kafka over direct HTTP between services:**

- **Availability decoupling**: If [[email-service]] is down during a purchase spike, the purchase response blocks, consumers wait, the system degrades. With Kafka, the purchase returns in <2s regardless of email service health. Messages persist until consumed.
- **Producer/consumer lifecycle independence**: Services can be deployed, scaled, and restarted independently. Kafka buffers messages during consumer downtime.
- **Enable future services**: New services can consume existing topics without modifying producers. `user.created` has no consumer yet, but any future service can start consuming it immediately.

## Implementation

### Topics

| Topic | Flow | Status |
|-------|------|--------|
| `user.created` | [[user-service]] → (future consumers) | Active producer, no consumer yet |
| `ticket.purchased` | [[ticket-service]] → [[email-service]] | Active |

### Message Design

Every message includes:
- `correlation_id` — traces a request across all services
- `idempotency_key` — prevents duplicate processing on the consumer side

### Consumer Pattern (Email Service Example)

1. Consume message from `ticket.purchased`
2. Extract `idempotency_key` (booking_ref)
3. Check `email_status` table for existing record with this key → skip if found
4. Process (fetch email, send, record status)
5. Commit Kafka offset

## Constitution Constraints

- "Services MUST communicate asynchronously for all non-blocking operations"
- "Fire-and-forget or event-driven patterns MUST be used for side effects such as sending emails, push notifications, and audit logging"
- "Synchronous HTTP calls between services are permitted ONLY for operations that require immediate consistency"
- "All asynchronous messages MUST be idempotent and include correlation IDs for traceability"

## What's NOT Async (by Design)

| Operation | Method | Why Synchronous |
|-----------|--------|-----------------|
| Ticket purchase (inventory decrement) | HTTP POST | Requires immediate consistency — must confirm or deny purchase before responding |
| Auth validation (Redis session lookup) | HTTP middleware | Must validate before processing request |

## Cross-references

- [[constitution]] — Principle III (Service Decoupling)
- [[kafka]] — the async backbone
- [[ticket-service]] — producer of purchase events
- [[email-service]] — consumer of purchase events
- [[user-service]] — producer of registration events
- [[trade-offs]] — v1 scope decisions about cross-service patterns
