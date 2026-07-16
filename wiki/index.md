# Wiki Index

Catalog of all pages in this wiki. Organized by category. Used by the LLM to locate relevant pages before drilling into them.

## Entities

| Page | Summary |
|------|---------|
| [[user-service]] | Identity, authentication, session management, PII encryption for user accounts |
| [[event-service]] | Public event catalog — browse upcoming events with pagination |
| [[ticket-service]] | Purchase flow with Redis-based distributed locking, atomic inventory decrement |
| [[email-service]] | Async confirmation delivery via Kafka consumer with idempotency and retry |
| [[kafka]] | Async event backbone — `user.created` and `ticket.purchased` topics |
| [[redis]] | Session store and distributed lock manager |
| [[mysql]] | Primary data store — 4 databases (user_db, event_db, ticket_db, email_db) |

## Concepts

| Page | Summary |
|------|---------|
| [[pii-encryption]] | AES-256-GCM column-level application encryption for all PII fields |
| [[distributed-locking]] | Redis SETNX per-event locks with Lua-script safe release, 30s TTL |
| [[service-decoupling]] | Kafka-based async messaging for non-blocking side effects |
| [[session-management]] | Redis-backed opaque session tokens with rolling TTL refresh |
| [[email-retry-strategy]] | Exponential backoff retry (1m→5m→15m→1h→4h) with dead-letter audit trail |
| [[testing-strategy]] | 112 tests across 3 levels: unit, integration, e2e. Zero Docker dependency at test time. |
| [[constitution]] | 4 governing principles (Security-First, Concurrency, Decoupling, TDD) |
| [[ci-cd-pipeline]] | GitHub Actions CI: build, test, Docker build+push to GHCR |
| [[trade-offs]] | Conscious v1 decisions and what was deferred to future scope |

## Source Maps

| Page | Summary |
|------|---------|
| [[sources/specs]] | Spec documents: functional requirements, plan, tasks, research, data model |
| [[sources/config-files]] | Infrastructure config: docker-compose, .env, Dockerfile, Makefile |
| [[sources/code-structure]] | Source code layout: services/, scripts/, shared packages |

## Overview

| Page | Summary |
|------|---------|
| [[overview]] | Wiki home — the big picture synthesis of the entire system |
