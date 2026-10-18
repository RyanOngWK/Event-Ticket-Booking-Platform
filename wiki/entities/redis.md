---
title: "Redis"
category: "entity"
tags: [infrastructure, cache, sessions, distributed-locking]
source_count: 8
updated: 2026-10-18
---

# Redis

## Overview

Redis serves as the **session store** and as the **Redlock lease layer** for ticket-purchase contention. Five independent Redis master processes are configured for the lock quorum; the first node is also used for sessions. Compose places all nodes on one host, so they are not production fault domains.

## Roles

### 1. Session Store

- Opaque session tokens stored as Redis keys with 24-hour TTL
- Rolling TTL refresh on authenticated activity
- Instant revocation on logout (DELETE key)
- Auth middleware reads `Authorization: Bearer <token>`, looks up in Redis, injects user context

### 2. Redlock Lease Layer

- Lock key pattern: lock:event:{event_id}
- Ticket Service coordinates a majority of five Redis masters with a 30-second lease.
- Each acquisition uses a unique owner value; release only removes that acquisition's own lock.
- MySQL fencing tokens, not Redis, reject stale booking writes after a higher token has been accepted.

## Configuration

- Docker Compose: session Redis at `redis:6379` plus four additional Redlock masters; all five nodes share one Compose host
- Used by: [[user-service]] (sessions), [[ticket-service]] (Redlock leases and session validation)

## Key Decisions

- **Redis over PostgreSQL advisory locks** — advisory locks tie the lock lifecycle to a database connection. In a microservice architecture, the lock holder may be a different process from the DB connection owner. Redis is connection-agnostic. See [[distributed-locking]].
- **Redis over etcd** — etcd would add a new infrastructure component. Redis is already in the stack for sessions.
- **30s TTL** — per [[constitution]] Principle II: "Lock TTL MUST be configured with a maximum of 30 seconds for ticket operations."

## Cross-references

- [[distributed-locking]] — detailed lock strategy and alternatives
- [[session-management]] — how session tokens work
- [[ticket-service]] — lock consumer
- [[user-service]] — session producer/validator
- [[constitution]] — Principle II (Concurrency Management)
- [[sources/config-files]] — docker-compose.yml Redis config
