---
title: "Email Retry Strategy"
category: "concept"
tags: [email, retry, kafka, idempotency, dead-letter, reliability]
source_count: 7
updated: 2025-07-16
---

# Email Retry Strategy

## Overview

The [[email-service]] uses exponential backoff for failed email deliveries, with a dead-letter mechanism after 5 failed attempts. This ensures that transient failures (SMTP timeout, rate limiting, network blip) are retried, while persistent failures (invalid email, provider outage) are surfaced for manual investigation.

## Retry Schedule

```mermaid
flowchart LR
    A["Attempt 1<br/>immediate"] --> B["Attempt 2<br/>+1m"]
    B --> C["Attempt 3<br/>+5m"]
    C --> D["Attempt 4<br/>+15m"]
    D --> E["Attempt 5<br/>+1h"]
    E --> F["Attempt 6<br/>+4h"]
    F --> DL["dead-letter"]
```

After 5 failures (6 total attempts including the initial), status transitions to `dead`. The record remains queryable in `email_db.email_status` for manual investigation.

### Timeout Provision

If an email delivery has not succeeded within 24 hours of creation, status transitions to `dead` regardless of retry count.

## Implementation

### Consumer Processing

```mermaid
flowchart TD
    A["Consume ticket.purchased"] --> B["Extract booking_ref"]
    B --> C{"Check email_status<br/>for booking_ref"}
    C -->|"found status=sent"| SKIP["Skip (idempotent)"]
    C -->|"found status=failed/pending"| D["Resume from state"]
    C -->|"not found"| D["Create new record<br/>status=pending"]
    D --> E["Fetch + decrypt user email<br/>from user_db"]
    E --> F["EmailProvider.Send()"]
    F -->|success| G["UPDATE status='sent'"]
    G --> H["Commit Kafka offset"]
    F -->|failure| I["UPDATE status='failed'<br/>retry_count += 1"]
    I --> J{"retry_count >= 5?"}
    J -->|yes| K["UPDATE status='dead'<br/>log correlation ID"]
    J -->|no| L["Do NOT commit offset<br/>schedule retry"]
    L --> A
```

### Idempotency

The `booking_ref` (from the `ticket.purchased` event) serves as the idempotency key. The consumer checks `email_status` for an existing record with that key before processing. This handles:
- Kafka at-least-once delivery (duplicate messages)
- Consumer restart before offset commit
- Manual retry of dead-lettered records

## Pluggable Provider Interface

```go
type EmailProvider interface {
    Send(ctx context.Context, to string, subject string, body string) error
}
```

Current implementation: `LogProvider` (stdout). Real SMTP/SendGrid/Mailgun providers are a deploy-time swap-in — no business logic changes needed.

## Dead-Letter Investigation

When a record reaches `dead` status:
- The correlation ID is logged for traceability
- The record persists in `email_db.email_status` with full failure history
- Manual investigation: query by `status='dead'`, review retry_count and last_attempt_at

## Cross-references

- [[email-service]] — implements this strategy
- [[kafka]] — message delivery and offset management
- [[ticket-service]] — producer of purchase events (booking_ref)
- [[pii-encryption]] — email decryption at read time
- [[constitution]] — Principle III (Service Decoupling), idempotency requirement
- [[trade-offs]] — dead-letter escalation decision
