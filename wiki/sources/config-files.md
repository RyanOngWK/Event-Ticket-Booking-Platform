---
title: "Source Map: Config Files"
category: "source-map"
tags: [sources, config, devops, build]
source_count: 6
updated: 2026-11-16
---

# Source Map: Configuration & Infrastructure Files

## Overview

Infrastructure configuration files define how the system runs, builds, and is deployed. These are the operational blueprints.

## Files

| File | Description | Maps to Wiki Pages |
|------|-------------|-------------------|
| `docker-compose.yml` | 4 application services + MySQL + one session Redis + Kafka + Zookeeper with health checks; `image:` field references GHCR | [[overview]], all entity pages, [[kafka]], [[redis]], [[mysql]], [[ci-cd-pipeline]] |
| `docker/Dockerfile` | Multi-stage Go build (glibc-based for confluent-kafka-go), parameterized by `SERVICE_NAME` arg | [[kafka]], [[trade-offs]], [[ci-cd-pipeline]] |
| `.github/workflows/ci.yml` | CI pipeline: go build/vet, unit/integration/e2e tests, Docker build+push to GHCR | [[ci-cd-pipeline]], [[testing-strategy]] |
| `.env.example` | Environment variable template (encryption key, DB passwords, ports, session Redis, Stripe test keys, reservation TTL) | [[pii-encryption]], [[mysql]], [[redis]], [[ticket-service]] |
| `Makefile` | up/down/build/test/seed, 10,000-client reservation simulation, and schema migration targets | [[testing-strategy]], [[ticket-service]], [[mysql]] |
| `scripts/db/init.sql` | 4 MySQL databases, tables, indexes, constraints, reservation-unit pool, timed reservations, ticket outbox | [[mysql]], [[reservation-unit-pool]], [[ticket-service]] |
| `scripts/db/migrations/002_event_fencing_tokens.sql` | Historical migration for the retired fencing-counter design | [[ticket-service]], [[mysql]] |
| `scripts/db/migrations/003_ticket_outbox.sql` | Idempotent outbox schema migration for existing MySQL volumes | [[ticket-service]], [[mysql]], [[service-decoupling]] |
| `scripts/kafka/init-topics.sh` | Topic bootstrap (user.created, ticket.purchased) | [[kafka]] |
| `scripts/seed/seed_events.go` | 6 sample events and their available/sold reservation units | [[event-service]], [[reservation-unit-pool]] |
| `scripts/db/migrations/004_reservation_units.sql` | Backfills one unit row per existing event capacity and removes fencing counters | [[ticket-service]], [[mysql]], [[reservation-unit-pool]] |
| `scripts/db/migrations/005_stripe_checkout.sql` | Adds event prices, reserved unit state, and timed checkout reservation table for existing volumes | [[ticket-service]], [[mysql]], [[reservation-unit-pool]] |
| `services/ticket/internal/payment/` | Stripe test-mode PaymentIntent creation/cancel/refund and signed webhook verification | [[ticket-service]], [[reservation-unit-pool]] |
| `services/ticket/internal/handler/checkout.html` | Local browser checkout using Stripe.js Payment Element and reservation polling | [[ticket-service]] |
| `.specify/memory/constitution.md` | Governing principles v1.1.0 | [[constitution]], [[reservation-unit-pool]] |
| `.specify/templates/` | Spec/plan/task templates | [[constitution]] |
| [Design PDF](/Users/ryanong/Desktop/Design%20a%20Flash%20Sale%20System%20%7C%20Hello%20Interview%20System%20Design%20in%20a%20Hurry.pdf) | Source design for per-unit inventory pools, row-lock claims, expiry, and admission control | [[reservation-unit-pool]], [[ticket-service]] |


## Ingest History

- **2025-07-17**: Ingest `.github/workflows/ci.yml` — documented CI pipeline with Docker build+push. Created [[ci-cd-pipeline]]. Updated index, log, trade-offs.
- **2025-07-16**: Full bootstrap — all config files ingested to create initial wiki pages. See [[log|log.md#2025-07-16-ingest-initial-wiki-bootstrap]].
