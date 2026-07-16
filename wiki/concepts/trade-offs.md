---
title: "Trade-offs & Future Scope"
category: "concept"
tags: [trade-offs, v1-scope, deferred, technical-debt, roadmap]
source_count: 7
updated: 2025-07-16
---

# Trade-offs & Future Scope

## Overview

Every architecture is a series of trade-offs. This page catalogs the conscious decisions made for v1 and what was deferred to future iterations. Many of these are tagged in the codebase as `TODO(v2)`.

## What's In Scope (v1)

- User registration with encrypted PII and session-based auth
- Public event catalog with pagination and sold-out indicators
- Concurrent ticket purchase with distributed locking (zero double-sells)
- Async confirmation email via Kafka with retry and dead-letter
- Rate limiting on auth endpoints (per-IP)
- Account deletion with PII anonymization (Singapore PDPA compliant)

## What's Deferred

| Area | v1 Decision | Why Deferred |
|------|-------------|--------------|
| **Payment processing** | Purchase = reservation only; no payment integration | Separates inventory from financial concern; payment adds PCI compliance scope |
| **Event management UI** | Events via seed data; no admin CRUD | Admin UX is a separate bounded context; seed data suffices for MVP |
| **Multi-factor auth** | Password-only | Adds SMS/OTP/TOTP infrastructure without changing core ticket flow |
| **Event search/filtering** | Pagination only; no keyword, date range, or venue filters | Premature optimization without production traffic patterns |
| **Multi-region deployment** | Single-region | Cross-region Redis lock coordination adds significant complexity |
| **OAuth/social login** | Email + password only | Adds third-party dependency; orthogonal to ticket purchase flow |
| **Kubernetes manifests** | Docker Compose for local dev | Compose sufficient for 4-service topology; K8s adds overhead |
| **gRPC between services** | Cross-service reads use shared MySQL | Premature optimization; future: add service-to-service APIs |
| **Production monitoring** | Structured logging with correlation IDs; no metrics/alerting | Logging provides debug trail; metrics require Prometheus/Grafana |
| **Configurable email provider** | LogProvider (stdout) implemented; real SMTP is a swap-in | Provider interface is pluggable; real config is a deploy-time concern |

## Known Technical Debt (Documented)

1. **Cross-database reads**: [[ticket-service]] reads `event_db` directly. Tagged `TODO(v2)` — should use Event Service API. See [[ticket-service#known-technical-debt]].
2. **glibc Docker dependency**: `confluent-kafka-go` C library requires Debian base image (glibc) instead of Alpine (musl). See [[kafka#known-technical-debt]].
3. **Lock granularity**: Lock-per-event could bottleneck under extreme demand on a single event. Sharded locks could address this.

## Design Decisions That Will Scale

These v1 choices were made with future scale in mind:

- **Dedicated databases per service** — extracting to separate MySQL instances (or different DB engines) is straightforward because schemas are already isolated. See [[mysql]].
- **Application-level encryption** — upgrading from env-var key to AWS KMS / HashiCorp Vault requires no code changes. See [[pii-encryption]].
- **Pluggable email provider** — swapping from LogProvider to real SMTP/SendGrid requires config only, no business logic changes. See [[email-retry-strategy]].
- **Kafka topics follow convention** — `user.created` topic exists with no consumer yet. Any future service can start consuming immediately. See [[kafka]], [[service-decoupling]].

## Cross-references

- [[overview]] — v1 system scope summary
- [[constitution]] — principles that these trade-offs respect
- [[ticket-service]] — cross-DB read technical debt
- [[kafka]] — glibc dependency
- [[pii-encryption]] — encryption upgrade path
- [[testing-strategy]] — all tests run without infrastructure
