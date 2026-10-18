---
title: "Distributed Locking"
category: "concept"
tags: [concurrency, locking, redis, race-condition, inventory]
source_count: 8
updated: 2026-10-18
---

# Distributed Locking

## Overview

Ticket Service uses a per-event Redlock lease for contention control and a MySQL fencing token to reject old writes after a newer token has been accepted. The database remains the authority for inventory.

## Rationale

Redlock coordinates access across five Redis masters. A majority can still be available when one master is unavailable. The Compose nodes share a host, so this demonstrates quorum behavior but not independent production failure domains.

Redlock provides a time-limited lease, not a fencing token. Ticket Service therefore allocates a monotonic token from MySQL before trying to acquire the lock. Requests that fail to acquire the lock can consume token values; gaps are harmless. Allocating before lock acquisition prevents a delayed request from resuming later and minting a higher token.

## Implementation

### Lock Acquisition

The service uses the Redsync Go library over five Redis masters. It makes one acquisition attempt per request with a 30-second expiry. Each lock has a unique owner value, and release targets only that value.

### Fencing Token

event_db.event_fencing_tokens stores last_issued_token and last_accepted_token per event. The Ticket Service increments the issued counter on a pinned MySQL connection before attempting Redlock. The conditional inventory update carries the token and updates inventory and last_accepted_token in the same SQL statement, only when the token exceeds the last accepted value.

Once MySQL has accepted a newer token, a delayed request with an older token affects zero rows and is rejected. This is the fencing boundary: the database remembers the newest token it has observed.

### Remaining Limits

Fencing does not make inventory decrement and ticket insertion atomic. A ticket insert can still fail after inventory changes. The next stage should put inventory, ticket, and an outbox row in one database transaction. Kafka delivery then happens asynchronously from the outbox.

## Constitution Constraints

- "Strict distributed locking MUST be implemented to prevent race conditions"
- "Lock TTL MUST be configured with a maximum of 30 seconds"
- "All ticket purchase workflows MUST acquire and release locks within bounded time limits"

## Cross-references

- [[constitution]] — Principle II (Concurrency Management)
- [[redis]] — lock manager
- [[ticket-service]] — lock consumer
- [[testing-strategy]] — concurrency stress test (100 goroutines, 1 ticket → exactly 1 wins)
- [[trade-offs]] — lock granularity considerations
- [[sources/research]] — design decision with alternatives analysis
