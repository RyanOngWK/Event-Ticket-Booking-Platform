---
title: "Source Map: Config Files"
category: "source-map"
tags: [sources, config, devops, build]
source_count: 3
updated: 2025-07-16
---

# Source Map: Configuration & Infrastructure Files

## Overview

Infrastructure configuration files define how the system runs, builds, and is deployed. These are the operational blueprints.

## Files

| File | Description | Maps to Wiki Pages |
|------|-------------|-------------------|
| `docker-compose.yml` | 4 services + MySQL + Redis + Kafka + Zookeeper with health checks | [[overview]], all entity pages, [[kafka]], [[redis]], [[mysql]] |
| `docker/Dockerfile` | Multi-stage Go build (glibc-based for confluent-kafka-go) | [[kafka]], [[trade-offs]] |
| `.env.example` | Environment variable template (encryption key, DB passwords, ports) | [[pii-encryption]], [[mysql]], [[redis]] |
| `Makefile` | up/down/build/test/seed targets | [[testing-strategy]], all entity pages |
| `scripts/db/init.sql` | 4 MySQL databases, all tables, indexes, constraints | [[mysql]], all entity data model sections |
| `scripts/kafka/init-topics.sh` | Topic bootstrap (user.created, ticket.purchased) | [[kafka]] |
| `scripts/seed/seed_events.go` | 6 sample events for local development | [[event-service]] |
| `.specify/memory/constitution.md` | Governing principles v1.0.0 | [[constitution]] |
| `.specify/templates/` | Spec/plan/task templates | [[constitution]] |

## Ingest History

- **2025-07-16**: Full bootstrap — all config files ingested to create initial wiki pages. See [[log|log.md#2025-07-16-ingest-initial-wiki-bootstrap]].
