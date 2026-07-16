---
title: "Source Map: Code Structure"
category: "source-map"
tags: [sources, code, go, services, structure]
source_count: 3
updated: 2025-07-16
---

# Source Map: Code Structure

## Overview

The source code is organized into four independent Go services with shared cross-cutting libraries. Each service owns its own bounded context and database. The full source tree is at `services/`.

## Directory Layout

```mermaid
flowchart LR
    subgraph services["services/"]
        direction TB
        subgraph shared["shared/pkg/"]
            sh_crypto["crypto/<br/>AES-256-GCM · SHA-256"]
            sh_kafka["kafka/<br/>producer · consumer"]
            sh_mw["middleware/<br/>auth · rate-limit · logging"]
        end
        svc_user["user/<br/>identity · auth"]
        svc_event["event/<br/>public catalog"]
        svc_ticket["ticket/<br/>purchase · locking"]
        svc_email["email/<br/>Kafka consumer"]
        svc_e2e["e2e/<br/>cross-service test"]
    end
    subgraph scripts["scripts/"]
        direction TB
        sc_db["db/init.sql"]
        sc_kafka["kafka/init-topics.sh"]
        sc_seed["seed/seed_events.go"]
    end
```

## Service Details

| Directory | Wiki Page | Key Files |
|-----------|-----------|-----------|
| `services/user/` | [[user-service]] | handler, service, repository, models |
| `services/event/` | [[event-service]] | handler, service, repository, models |
| `services/ticket/` | [[ticket-service]] | handler, service, repository, models, lock manager |
| `services/email/` | [[email-service]] | consumer handler, email provider interface, status repository |
| `services/shared/pkg/crypto/` | [[pii-encryption]] | encrypt, decrypt, hash functions |
| `services/shared/pkg/kafka/` | [[kafka]], [[service-decoupling]] | producer, consumer, event envelope types |
| `services/shared/pkg/middleware/` | [[session-management]] | auth middleware (Redis lookup), rate limiter, request logging |

## Test Files

Tests are co-located with their source:

| Service | Unit Tests | Integration Tests | Wiki Page |
|---------|-----------|-------------------|-----------|
| User | `services/user/*_test.go` | `services/user/*_test.go` | [[user-service]], [[testing-strategy]] |
| Event | `services/event/*_test.go` | `services/event/*_test.go` | [[event-service]], [[testing-strategy]] |
| Ticket | `services/ticket/*_test.go` | `services/ticket/*_test.go` | [[ticket-service]], [[testing-strategy]], [[distributed-locking]] |
| Email | `services/email/*_test.go` | `services/email/*_test.go` | [[email-service]], [[testing-strategy]], [[email-retry-strategy]] |

## Ingest History

- **2025-07-16**: Full bootstrap — code structure cataloged. Individual service source files not yet deeply ingested — pending future ingest operations. See [[log|log.md#2025-07-16-ingest-initial-wiki-bootstrap]].
