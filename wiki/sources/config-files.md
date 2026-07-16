---
title: "Source Map: Config Files"
category: "source-map"
tags: [sources, config, devops, build]
source_count: 4
updated: 2025-07-17
---

# Source Map: Configuration & Infrastructure Files

## Overview

Infrastructure configuration files define how the system runs, builds, and is deployed. These are the operational blueprints.

## Files

| File | Description | Maps to Wiki Pages |
|------|-------------|-------------------|
| `docker-compose.yml` | 4 services + MySQL + Redis + Kafka + Zookeeper with health checks; `image:` field references GHCR | [[overview]], all entity pages, [[kafka]], [[redis]], [[mysql]], [[ci-cd-pipeline]] |
| `docker/Dockerfile` | Multi-stage Go build (glibc-based for confluent-kafka-go), parameterized by `SERVICE_NAME` arg | [[kafka]], [[trade-offs]], [[ci-cd-pipeline]] |
| `.github/workflows/ci.yml` | CI pipeline: go build/vet, unit/integration/e2e tests, Docker build+push to GHCR | [[ci-cd-pipeline]], [[testing-strategy]] |
| `.env.example` | Environment variable template (encryption key, DB passwords, ports) | [[pii-encryption]], [[mysql]], [[redis]] |
| `Makefile` | up/down/build/test/seed targets | [[testing-strategy]], all entity pages |
| `scripts/db/init.sql` | 4 MySQL databases, all tables, indexes, constraints | [[mysql]], all entity data model sections |
| `scripts/kafka/init-topics.sh` | Topic bootstrap (user.created, ticket.purchased) | [[kafka]] |
| `scripts/seed/seed_events.go` | 6 sample events for local development | [[event-service]] |
| `.specify/memory/constitution.md` | Governing principles v1.0.0 | [[constitution]] |
| `.specify/templates/` | Spec/plan/task templates | [[constitution]] |

## Ingest History

- **2025-07-17**: Ingest `.github/workflows/ci.yml` — documented CI pipeline with Docker build+push. Created [[ci-cd-pipeline]]. Updated index, log, trade-offs.
- **2025-07-16**: Full bootstrap — all config files ingested to create initial wiki pages. See [[log|log.md#2025-07-16-ingest-initial-wiki-bootstrap]].
