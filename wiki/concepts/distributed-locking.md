---
title: "Distributed Locking"
category: "concept"
tags: [concurrency, locking, redis, race-condition, inventory]
source_count: 7
updated: 2025-07-16
---

# Distributed Locking

## Overview

The ticket purchase flow must serialize concurrent requests to prevent double-selling — the classic "overselling" problem. The solution is a Redis-based distributed lock scoped per event. This is a MUST principle under the [[constitution]] (Principle II: Concurrency Management).

## Rationale

**Why Redis-based distributed lock over PostgreSQL advisory locks:**

- PostgreSQL advisory locks tie the lock lifecycle to a database connection. In a microservice architecture, the lock holder may be a different process from the DB connection owner — the lock becomes unreliable.
- Redis is connection-agnostic — the lock is independent of any database connection.
- Redis is already in the stack for [[session-management]] — no new infrastructure.
- Purpose-built for distributed mutual exclusion.

**Why lock-per-event, not lock-per-ticket:**

- Locking at the event granularity is sufficient for ticket inventory management.
- Per-ticket locking would require managing thousands of locks per event — complexity without benefit.
- If extreme contention on a single event becomes a bottleneck, sharded locks can be introduced later.

## Implementation

### Lock Acquisition

```
SET lock:event:{event_id} {instance_id} NX EX 30
```

- `NX` — "set if Not eXists" → atomic lock acquisition. Only one instance succeeds.
- `EX 30` — 30-second TTL. If the lock holder crashes, the lock auto-expires (no deadlock).
- `instance_id` — unique per service instance, used to verify ownership on release.

### Lock Release (Safe)

Lua script executed atomically on Redis:

```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
```

This prevents an instance from releasing another instance's lock (e.g., if the original lock expired and a new lock was acquired).

### Retry Strategy

If lock acquisition fails (NX returns nil), the client retries with exponential backoff:
- 3 attempts: 50ms → 100ms → 200ms
- After 3 failures → HTTP 423 Locked, client should retry with its own backoff

### Dual Safety

Even if the distributed lock somehow fails, the SQL UPDATE provides a second layer of protection:
```sql
UPDATE events SET remaining_count = remaining_count - ? 
WHERE id = ? AND remaining_count >= ?
```
If `remaining_count` has already dropped below the requested quantity, zero rows are affected and the purchase is rejected.

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
