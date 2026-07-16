---
title: "Redis"
category: "entity"
tags: [infrastructure, cache, sessions, distributed-locking]
source_count: 7
updated: 2025-07-16
---

# Redis

## Overview

Redis serves dual roles in this architecture: **session store** for authentication and **distributed lock manager** for ticket purchase concurrency control. It was already in the stack for sessions, so adding locking didn't introduce a new infrastructure component.

## Roles

### 1. Session Store

- Opaque session tokens stored as Redis keys with 24-hour TTL
- Rolling TTL refresh on authenticated activity
- Instant revocation on logout (DELETE key)
- Auth middleware reads `Authorization: Bearer <token>`, looks up in Redis, injects user context

### 2. Distributed Lock Manager

- Lock key pattern: `lock:event:{event_id}`
- Implementation: `SET lock:event:{id} {instance_id} NX EX 30`
  - `NX` — only set if key doesn't exist (atomic lock acquisition)
  - `EX 30` — 30-second TTL prevents deadlock if lock holder crashes
- Safe release: Lua script checks that the value matches before deleting (prevents releasing another instance's lock)
- Retry with exponential backoff: 3 attempts (50ms, 100ms, 200ms) before returning HTTP 423

## Configuration

- Docker Compose: Redis container on `redis:6379`
- Used by: [[user-service]] (sessions), [[ticket-service]] (locks, also validates sessions)

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
