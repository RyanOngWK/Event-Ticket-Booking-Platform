---
title: "Project Constitution"
category: "concept"
tags: [governance, principles, security, concurrency, decoupling, tdd]
source_count: 2
updated: 2025-07-16
---

# Project Constitution

## Overview

The constitution is the immutable governing document for all development. Four non-negotiable principles constrain every downstream decision — spec, plan, tasks, implementation, review. Any PR that violates a MUST principle is rejected. The constitution is versioned (semver) and amended through a documented process.

**Source**: [constitution.md](../.specify/memory/constitution.md)

**Version**: 1.0.0 | **Ratified**: 2025-07-14

## The Four Principles

### I. Security-First (NON-NEGOTIABLE)
- PII encrypted at rest using AES-256-GCM (see [[pii-encryption]])
- Encryption keys NOT in source code or config files
- Plaintext PII NEVER in logs, errors, or analytics
- Services handling PII require security review before deployment

### II. Concurrency Management
- Strict distributed locking for all ticket purchases (see [[distributed-locking]])
- Lock TTL maximum 30 seconds
- Bounded time limits on lock acquisition and release

### III. Service Decoupling
- Async messaging (Kafka) for all non-blocking operations (see [[service-decoupling]])
- Synchronous HTTP ONLY for immediate consistency requirements
- All async messages MUST be idempotent with correlation IDs

### IV. Test-Driven Development (NON-NEGOTIABLE)
- Unit AND integration tests before deployment (see [[testing-strategy]])
- TDD: Red → Green → Refactor
- 80% minimum code coverage on core business logic
- All tests must pass in CI before merge

## Technical Constraints

- **Encryption**: AES-256-GCM column-level or application-level
- **Message Broker**: Apache Kafka, topics follow `<domain>.<event>` naming
- **Distributed Locking**: Redis Redlock or PostgreSQL advisory locks
- **Containerization**: Docker Compose for local, K8s for production (deferred)

## Development Gates

1. **Pre-Commit**: Lint + format + unit tests for changed services
2. **Pull Request**: Unit + integration tests, CI pass, security scan
3. **Code Review**: At least one approval, must verify constitution compliance
4. **Deployment**: No deployment without passing CI, staging validation before production

## Governance

Amendments require:
1. Clear rationale
2. Impact assessment on existing services
3. Migration plan if affecting running systems
4. Project lead/architecture owner approval

## Cross-references

- [[pii-encryption]] — Principle I implementation
- [[distributed-locking]] — Principle II implementation
- [[service-decoupling]] — Principle III implementation
- [[testing-strategy]] — Principle IV implementation
- [[trade-offs]] — v1 scope decisions made within constitutional constraints
- [[overview]] — how the constitution shapes the system architecture
