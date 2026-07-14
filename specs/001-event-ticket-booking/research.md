# Research: Event Ticket Booking Platform

**Date**: 2025-07-14

## Decision Log

### 1. PII Encryption Strategy

- **Decision**: Application-level encryption using AES-256-GCM with per-user encryption keys
  derived from a master key.
- **Rationale**: Column-level encryption in MySQL requires enterprise edition or external
  plugins. Application-level encryption keeps the solution database-agnostic and gives full
  control over key management. AES-256-GCM provides authenticated encryption (confidentiality
  + integrity). Per-user keys limit blast radius if a key is compromised.
- **Alternatives considered**:
  - MySQL TDE (Transparent Data Encryption): Rejected — requires MySQL Enterprise Edition,
    encrypts at file level not column level, keys managed by MySQL.
  - AWS KMS envelope encryption: Deferred — adds cloud dependency for v1. Architecture
    supports upgrading to KMS later.

### 2. Distributed Locking Implementation

- **Decision**: Redis Redlock algorithm via go-redis, with lock-per-event-key pattern.
- **Rationale**: Redlock is the most widely adopted Redis-based distributed lock algorithm.
  Lock key = `lock:event:{event_id}`. Each purchase request acquires this lock, checks
  inventory, decrements if available, then releases. 30s TTL prevents stuck locks. Retry with
  exponential backoff (3 attempts, 50ms/100ms/200ms) before returning "ticket unavailable".
- **Alternatives considered**:
  - PostgreSQL advisory locks: Rejected — ties locking to the database tier, adds latency
    under high concurrency, less suitable for microservices where the lock holder may not own
    the DB connection.
  - etcd distributed lock: Rejected — introduces an additional infrastructure component;
    Redis is already in the stack for sessions.

### 3. Kafka Topic Design

- **Decision**: Two topics following `<domain>.<event>` convention per constitution:
  `user.created` and `ticket.purchased`.
- **Rationale**: `user.created` enables future services (welcome emails, analytics) to react
  to registrations. `ticket.purchased` drives the Email Service for confirmation emails and
  provides an audit trail. Each message includes `correlation_id` for request tracing and
  `idempotency_key` to prevent duplicate processing.
- **Alternatives considered**:
  - Single topic with type field: Rejected — violates single-responsibility, harder to scale
    consumers independently.
  - Direct HTTP calls between services: Rejected — violates constitution Principle III
    (service decoupling), blocks the purchase flow.

### 4. Go Web Framework

- **Decision**: `net/http` standard library with `gorilla/mux` router.
- **Rationale**: `gorilla/mux` is a mature, production-tested Go router with route variables,
  middleware chaining, and method-based routing. It provides a clean API while staying close to
  stdlib conventions.
- **Alternatives considered**:
  - Gin: Rejected — non-standard handler signatures.
  - Echo: Rejected — similar concerns to Gin; gorilla/mux is more lightweight.
  - chi: Rejected — gorilla/mux was already adopted in the codebase.

### 5. Session Management

- **Decision**: Redis-backed session tokens with 24-hour expiry and rolling refresh.
- **Rationale**: Stateless services are critical in a microservices architecture. Session
  tokens stored in Redis allow any service instance to validate authentication without sticky
  sessions. The auth middleware reads the `Authorization: Bearer <token>` header, looks up
  the session in Redis, and injects the user context.
- **Alternatives considered**:
  - JWT stateless tokens: Rejected — no built-in revocation mechanism without a blocklist
    (which requires Redis anyway), larger header payloads.
  - Database-stored sessions: Rejected — adds read load to MySQL for every authenticated
    request; Redis is purpose-built for this.

### 6. Email Sending & Retry Strategy

- **Decision**: Kafka consumer in Email Service reads `ticket.purchased` events, dispatches
  to a pluggable email provider interface, and tracks delivery status in MySQL. Retry with
  exponential backoff: 1m → 5m → 15m → 1h → 4h, then dead-letter.
- **Rationale**: Kafka consumer offset management guarantees at-least-once delivery. The
  email status table in MySQL ensures no confirmation is lost even if the service restarts.
  Pluggable provider interface allows swapping between SMTP, SendGrid, Mailgun, etc. without
  changing business logic.
- **Alternatives considered**:
  - Kafka Streams for retry: Rejected — adds complexity; manual consumer with offset commit
    after successful send is simpler and sufficient.

### 7. Database Schema Design

- **Decision**: Dedicated MySQL database per service with shared instance. Encryption keys
  stored in environment variables, not in the database.
- **Rationale**: Per-service schemas (or databases) provide logical isolation and make future
  extraction to separate instances straightforward. The Ticket Service owns the `tickets`
  table and reads `events.remaining_tickets` from the Event Service's database via a shared
  read or internal API. For v1 simplicity, services share a MySQL instance with separate
  databases/schemas.
- **Alternatives considered**:
  - Shared single database/schema: Rejected — tight coupling, harder to split later.
  - Separate MySQL instances per service: Rejected — overkill for v1; Docker Compose runs a
    single MySQL container with multiple databases.

### 8. Docker Compose vs Kubernetes for Local Dev

- **Decision**: Docker Compose for local development and CI testing. Kubernetes manifests
  deferred to production readiness.
- **Rationale**: Docker Compose provides a single-command local setup (`docker compose up`).
  It supports health checks, depends_on with conditions, and volume mounts for hot-reload.
  Kubernetes would add significant complexity (minikube, kubectl, ingress) with no benefit for
  local development.
- **Alternatives considered**:
  - Tilt/Skaffold: Rejected — adds tooling overhead for a 4-service system; Docker Compose
    with Compose Watch is sufficient.
