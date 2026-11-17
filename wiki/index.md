# Wiki Index

Catalog of all pages in this wiki. Organized by category. Used by the LLM to locate relevant pages before drilling into them.

## Entities

| Page | Summary |
|------|---------|
| [[user-service]] | Identity, authentication, session management, PII encryption for user accounts |
| [[event-service]] | Public event catalog — browse upcoming events with pagination |
| [[ticket-service]] | Timed reservation lifecycle with Stripe test checkout, verified webhook settlement, and outbox |
| [[email-service]] | Async confirmation delivery via Kafka consumer with idempotency and retry |
| [[kafka]] | Async event backbone — `user.created` and `ticket.purchased` topics |
| [[redis]] | Session store for authentication; not used for ticket inventory coordination |
| [[mysql]] | Primary data store — 4 databases (user_db, event_db, ticket_db, email_db) |

## Concepts

| Page | Summary |
|------|---------|
| [[pii-encryption]] | AES-256-GCM column-level application encryption for all PII fields |
| [[reservation-unit-pool]] | Per-unit MySQL inventory holds, on-demand expiry reclaim, and payment settlement with `FOR UPDATE SKIP LOCKED` |
| [[service-decoupling]] | Kafka-based async messaging for non-blocking side effects |
| [[session-management]] | Redis-backed opaque session tokens with rolling TTL refresh |
| [[email-retry-strategy]] | Exponential backoff retry (1m→5m→15m→1h→4h) with dead-letter audit trail |
| [[testing-strategy]] | Unit, integration, and end-to-end checks plus a manual 10,000-client reservation burst using real MySQL |
| [[kafka-producer]] | Code walkthrough of the sending side — shared Producer, publishers, envelope format |
| [[kafka-consumer]] | Code walkthrough of the reading side — Consume loop, offset commits, email handler |
| [[constitution]] | 4 governing principles (Security-First, Concurrency, Decoupling, TDD) |
| [[ci-cd-pipeline]] | GitHub Actions CI: build, test, Docker build+push to GHCR |
| [[trade-offs]] | Resume-ready architecture decision record: v1 benefits, costs, code gaps, and production evolution |
| [[interview-tradeoffs]] | Interview guide to the flash-sale design decisions, assumptions, alternatives, limits, and scale-up paths |

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
