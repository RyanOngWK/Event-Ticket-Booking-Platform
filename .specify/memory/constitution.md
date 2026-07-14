<!--
Sync Impact Report
==================
Version change: 0.0.0 (template/placeholder) → 1.0.0
Bump rationale: MAJOR — first substantive constitution filling all placeholder tokens
  with governing principles, technical constraints, workflow gates, and governance rules.

Modified principles:
  [PRINCIPLE_1_NAME] → I. Security-First
  [PRINCIPLE_2_NAME] → II. Concurrency Management
  [PRINCIPLE_3_NAME] → III. Service Decoupling
  [PRINCIPLE_4_NAME] → IV. Test-Driven Development
  [PRINCIPLE_5_NAME] → Removed (user specified exactly 4 principles)

Added sections:
  - Technical Constraints (replaces [SECTION_2])
  - Development Workflow & Quality Gates (replaces [SECTION_3])

Removed sections:
  - [PRINCIPLE_5_NAME] slot (not needed)

Templates requiring updates:
  - .specify/templates/plan-template.md: ✅ No update needed (Constitution Check is generic)
  - .specify/templates/spec-template.md: ✅ No update needed (no direct references)
  - .specify/templates/tasks-template.md: ⚠ Pending — line 12 states "Tests are OPTIONAL"
    but Principle IV makes testing non-negotiable. Header and test-phase guidance should be
    updated to reflect mandatory testing.
  - .specify/templates/checklist-template.md: ✅ No update needed

Follow-up TODOs: None
-->

# Microservices Platform Constitution

## Core Principles

### I. Security-First (NON-NEGOTIABLE)

All Personally Identifiable Information (PII), including but not limited to email addresses,
MUST be encrypted at rest in the database. Encryption MUST use industry-standard algorithms
(AES-256-GCM or equivalent). Encryption keys MUST be managed via a secure key management
service and MUST NOT be stored in source code or configuration files. Any service handling PII
MUST undergo a security review before deployment. Plaintext PII MUST never appear in logs,
error messages, or analytics pipelines.

### II. Concurrency Management

Strict distributed locking MUST be implemented to prevent race conditions during ticket
purchasing operations. The system MUST use a distributed lock manager (e.g., Redis Redlock,
PostgreSQL advisory locks, or equivalent) to ensure that only one transaction can modify a
ticket's availability state at any given time. All ticket purchase workflows MUST acquire and
release locks within bounded time limits to prevent deadlocks. Lock TTL MUST be configured
with a maximum of 30 seconds for ticket operations.

### III. Service Decoupling

Services MUST communicate asynchronously for all non-blocking operations. Fire-and-forget or
event-driven patterns (e.g., Apache Kafka, RabbitMQ) MUST be used for side effects such as
sending emails, push notifications, and audit logging. Synchronous HTTP calls between services
are permitted ONLY for operations that require immediate consistency (e.g., payment
authorization). All asynchronous messages MUST be idempotent and include correlation IDs for
traceability.

### IV. Test-Driven Development (NON-NEGOTIABLE)

All core business logic MUST be covered by unit tests AND integration tests before deployment.
Tests MUST be written before implementation (TDD: Red → Green → Refactor). Unit tests MUST
cover individual service logic in isolation, with mocked external dependencies. Integration
tests MUST cover inter-service communication contracts and database interactions. Test suites
MUST pass in the CI/CD pipeline before any merge to the main branch. Minimum code coverage
for core business logic: 80%.

## Technical Constraints

- **Encryption**: All PII fields MUST use column-level encryption or application-level
  encryption before writing to the database.
- **Message Broker**: Apache Kafka is the required message broker for asynchronous
  inter-service communication. Topics MUST follow the naming convention
  `<domain>.<event>` (e.g., `ticket.purchased`, `email.send`).
- **Distributed Locking**: Redis Redlock or PostgreSQL advisory locks are the approved
  distributed locking mechanisms.
- **Containerization**: All services MUST be containerized (Docker). Orchestration via
  Kubernetes or Docker Compose for local development.

## Development Workflow & Quality Gates

- **Pre-Commit**: Linting and formatting checks MUST pass. Pre-commit hooks MUST run
  unit tests for changed services.
- **Pull Request**: All PRs MUST include unit and integration tests for new or modified
  business logic. PRs MUST pass CI/CD including test suite, security scan, and lint checks.
- **Code Review**: At least one approving review is required. Reviewer MUST verify
  compliance with all constitution principles before approval.
- **Deployment**: No deployment without passing CI/CD. Staging environment validation
  is required before production deployment.
- **Code Coverage**: Core business logic MUST maintain at least 80% test coverage.
  Coverage reports are generated on every CI run.

## Governance

This constitution supersedes all other development practices and conventions. Any amendment
to this constitution MUST be documented with:

1. A clear rationale for the change.
2. An impact assessment on existing services.
3. A migration plan if the change affects running systems.
4. Approval from the project lead or architecture owner.

Compliance with this constitution is MANDATORY for all contributors. Pull requests that
violate any principle MUST be rejected. Complexity that deviates from these principles MUST
be justified in the implementation plan and approved before development begins.

All PRs and code reviews MUST verify compliance with the principles defined in this document.
Use the plan-template Constitution Check as a pre-implementation gate.

**Version**: 1.0.0 | **Ratified**: 2025-07-14 | **Last Amended**: 2025-07-14
