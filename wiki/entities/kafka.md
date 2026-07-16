---
title: "Apache Kafka"
category: "entity"
tags: [infrastructure, messaging, async, event-driven]
source_count: 7
updated: 2025-07-16
---

# Apache Kafka

## Overview

Kafka is the async event backbone connecting services. It decouples producer and consumer lifecycles — a service can publish an event without knowing or caring if the consumer is running. This is the mechanism for Principle III of the [[constitution]] (Service Decoupling).

## Topics

Following the `<domain>.<event>` naming convention from the [[constitution]]:

| Topic | Producer | Consumer | Purpose |
|-------|----------|----------|---------|
| `user.created` | [[user-service]] | None yet (future) | Enables future services (welcome emails, analytics) to react to new registrations |
| `ticket.purchased` | [[ticket-service]] | [[email-service]] | Triggers async confirmation email delivery |

## Message Format

Every message includes:
- `correlation_id` — for request tracing across services
- `idempotency_key` — to prevent duplicate processing
- Event-specific payload (user data for `user.created`, purchase data for `ticket.purchased`)

## Configuration

- Bootstrap via `scripts/kafka/init-topics.sh` — creates topics on startup
- Docker Compose: Kafka + Zookeeper containers
- Consumer: `confluent-kafka-go` library (vendored C library — the only non-Go-native component)

## Key Decisions

- **Kafka over direct HTTP** — synchronous HTTP couples service availability. If [[email-service]] is down during a purchase spike, the purchase response blocks, consumers wait, and the system degrades. Kafka decouples: purchase returns in <2s regardless of email service health. See [[service-decoupling]].
- **At-least-once delivery** with consumer offset management. Idempotency handled at the consumer level (check `email_status` for duplicate `booking_ref`).
- **Two topics, not one with type field** — single-responsibility per topic, consumers can scale independently.

## Known Technical Debt

- The `confluent-kafka-go` library depends on a vendored C library (`librdkafka`). This is the only non-Go-native component in the stack. The Dockerfile uses Debian base image (glibc) instead of Alpine (musl) to avoid linker issues.

## Cross-references

- [[service-decoupling]] — architectural rationale for Kafka
- [[email-service]] — consumer of `ticket.purchased`
- [[ticket-service]] — producer of `ticket.purchased`
- [[user-service]] — producer of `user.created`
- [[constitution]] — Principle III, topic naming convention
- [[sources/config-files]] — docker-compose.yml and init-topics.sh
