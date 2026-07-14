# Tasks: Event Ticket Booking Platform

**Input**: Design documents from `/specs/001-event-ticket-booking/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are MANDATORY per Constitution Principle IV (Test-Driven Development). All core
business logic requires unit + integration tests before deployment. Minimum 80% coverage.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Microservices monorepo**: `services/<name>/cmd/`, `services/<name>/internal/`, `services/<name>/tests/`
- **Shared code**: `services/shared/pkg/`
- **Infrastructure**: root-level `docker-compose.yml`, `scripts/`, `Makefile`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, Docker infrastructure, and tooling

- [x] T001 Create project directory structure for all 4 services and shared packages per plan.md structure in `services/`
- [x] T002 [P] Create `docker-compose.yml` with MySQL 8.0, Redis 7, Kafka (with Zookeeper), and health checks for all containers
- [x] T003 [P] Create MySQL initialization scripts (`scripts/db/init.sql`) that create `user_db`, `event_db`, `ticket_db`, `email_db` databases and all tables per data-model.md
- [x] T004 Initialize Go module at repo root (`go mod init github.com/example/ticket-platform`) in `go.mod`
- [x] T005 [P] Create `.env.example` with all required environment variables (DB creds, Redis URL, Kafka brokers, encryption key, service ports)
- [x] T006 [P] Create reusable Go service Dockerfile with multi-stage build in `docker/Dockerfile`
- [x] T007 [P] Create `Makefile` stub with targets: `up`, `down`, `build`, `test-unit`, `test-integration`, `test-concurrency`, `seed`
- [x] T008 [P] Create Kafka topic bootstrap script that creates `user.created` and `ticket.purchased` topics on startup in `scripts/kafka/init-topics.sh`

**Checkpoint**: `docker compose up` starts MySQL, Redis, Kafka, and Zookeeper successfully.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared packages that ALL services depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T009 Implement AES-256-GCM encryption/decryption package (Encrypt/Decrypt functions with key derivation) in `services/shared/pkg/crypto/crypto.go`
- [x] T010 [P] Write unit tests for crypto package (encrypt round-trip, tamper detection, empty input, key validation) in `services/shared/pkg/crypto/crypto_test.go`
- [x] T011 [P] Implement Kafka producer helper (connect, publish with envelope, correlation ID, JSON serialization) in `services/shared/pkg/kafka/producer.go`
- [x] T012 [P] Implement Kafka consumer helper (subscribe, poll, commit offsets, error handling) in `services/shared/pkg/kafka/consumer.go`
- [x] T013 [P] Implement correlation ID middleware (extract or generate X-Correlation-ID header, inject into context) in `services/shared/pkg/middleware/correlation.go`
- [x] T014 [P] Implement structured request logging middleware (method, path, status, duration, correlation ID) in `services/shared/pkg/middleware/logging.go`
- [x] T015 Implement Redis session authentication middleware (validate Bearer token, lookup session in Redis, inject user context) in `services/shared/pkg/middleware/auth.go`
- [x] T016 [P] Write unit tests for auth middleware (valid token, expired token, missing header, invalid format) in `services/shared/pkg/middleware/auth_test.go`

**Checkpoint**: Foundation ready — shared packages are tested and all user story implementation can now begin.

---

## Phase 3: User Story 1 - User Registration with Secure Data (Priority: P1) 🎯 MVP

**Goal**: Visitors can create accounts with encrypted PII storage, log in, and log out. A
`user.created` event is published to Kafka on successful registration.

**Independent Test**: Submit registration form → verify login works with same credentials →
verify email/name are binary encrypted blobs in MySQL (not plaintext) → verify `user.created`
Kafka message is published.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [P] [US1] Write unit tests for User repository (Create, FindByEmail, FindByID) in `services/user/tests/unit/user_repo_test.go`
- [x] T018 [P] [US1] Write unit tests for auth service (Register with valid data, duplicate email rejection, Login with correct/incorrect credentials, password hashing) in `services/user/tests/unit/auth_service_test.go`
- [x] T019 [P] [US1] Write integration tests for User Service HTTP handlers (POST /register success/duplicate/validation, POST /login success/failure, POST /logout, GET /me) in `services/user/tests/integration/handler_test.go`

### Implementation for User Story 1

- [x] T020 [P] [US1] Create User model struct (ID, Name, Email, PasswordHash, CreatedAt, UpdatedAt) in `services/user/internal/model/user.go`
- [x] T021 [US1] Implement User repository (Create with encrypted name/email + email hash, FindByEmailHash, FindByID) in `services/user/internal/repository/user_repo.go`
- [x] T022 [US1] Implement auth service (Register: validate inputs, check email uniqueness via hash, encrypt PII with crypto package, bcrypt password, insert user, publish user.created event) in `services/user/internal/service/auth_service.go`
- [x] T023 [US1] Implement User Service HTTP handlers (POST /register, POST /login, POST /logout, GET /me with session auth) in `services/user/internal/handler/handler.go`
- [x] T024 [US1] Implement Kafka publisher for `user.created` event (envelope with correlation ID, user_id payload) in `services/user/internal/publisher/publisher.go`
- [x] T025 [US1] Create User Service entry point (wire DB connection, Redis client, Kafka producer, router with middleware, start HTTP server on port 8081) in `services/user/cmd/main.go`
- [x] T026 [US1] Add User Service to docker-compose.yml with environment variables, port 8081, depends_on MySQL/Redis/Kafka, and health check

**Checkpoint**: User Story 1 is fully functional — register, login, logout, encrypted PII at rest, `user.created` events published to Kafka.

---

## Phase 4: User Story 2 - Browse Available Events (Priority: P2)

**Goal**: Any visitor (authenticated or not) can browse upcoming events and view details.
Sold-out events are clearly marked.

**Independent Test**: Seed events via script → GET /events without auth header → verify all
events returned with correct remaining_count → GET /events/:id returns full details → sold-out
events show `sold_out: true`.

### Tests for User Story 2

- [x] T027 [P] [US2] Write unit tests for Event repository (FindAll, FindByID, FindUpcoming with cursor pagination) in `services/event/tests/unit/event_repo_test.go`
- [x] T028 [P] [US2] Write unit tests for Event service (List with pagination, GetDetail, sold-out flag logic) in `services/event/tests/unit/event_service_test.go`
- [x] T029 [P] [US2] Write integration tests for Event Service handlers (GET /events with/without pagination, GET /events/:id exists/not-found, sold-out event response) in `services/event/tests/integration/handler_test.go`

### Implementation for User Story 2

- [x] T030 [P] [US2] Create Event model struct (ID, Name, Description, Date, Venue, TotalCapacity, RemainingCount, CreatedAt, UpdatedAt) in `services/event/internal/model/event.go`
- [x] T031 [US2] Implement Event repository (FindUpcoming with pagination, FindByID, count-filter for sold-out) in `services/event/internal/repository/event_repo.go`
- [x] T032 [US2] Implement Event service (List: paginate upcoming events with SoldOut flag derived from RemainingCount==0; GetDetail: full event by ID) in `services/event/internal/service/event_service.go`
- [x] T033 [US2] Implement Event Service HTTP handlers (GET /events with page/per_page query params, GET /events/:id — both public, no auth required) in `services/event/internal/handler/handler.go`
- [x] T034 [US2] Create Event Service entry point (wire DB connection, router with logging/correlation middleware, no auth middleware needed, start HTTP server on port 8082) in `services/event/cmd/main.go`
- [x] T035 [US2] Create event seed data script (insert 5+ sample events with varying remaining counts including one sold-out) in `scripts/seed/seed_events.go`
- [x] T036 [US2] Add Event Service to docker-compose.yml with environment variables, port 8082, depends_on MySQL, and health check

**Checkpoint**: User Story 2 is fully functional — public event browsing, pagination, sold-out display, seed data available.

---

## Phase 5: User Story 3 - Concurrent Ticket Purchase (Priority: P3)

**Goal**: Authenticated users can purchase tickets with Redis-backed distributed locking
guaranteeing zero double-sells under concurrent load. Confirmation emails are sent
asynchronously via Kafka. Users can view purchase history.

**Independent Test**: Create event with 1 ticket → fire 50 concurrent purchases → exactly 1
succeeds (201), 49 are declined (409/423) → remaining_count is 0 → tickets table has 1 row →
email_status row exists with status 'sent' → GET /tickets returns the purchase.

### Tests for User Story 3

- [x] T037 [P] [US3] Write unit tests for Ticket repository (Create, FindByUserID, FindByBookingRef, booking_ref uniqueness) in `services/ticket/tests/unit/ticket_repo_test.go`
- [x] T038 [P] [US3] Write unit tests for Redis distributed lock (acquire success, acquire failure when held, release, TTL expiry, re-entrant acquire rejection) in `services/ticket/tests/unit/redis_lock_test.go`
- [x] T039 [P] [US3] Write unit tests for purchase service (successful purchase, insufficient tickets, event not found, past event, zero/negative quantity, lock contention, event row decrement atomicity) in `services/ticket/tests/unit/purchase_service_test.go`
- [x] T040 [P] [US3] Write integration tests for Ticket Service handlers (POST /tickets/purchase success, POST /tickets/purchase sold-out, POST /tickets/purchase past-event, POST /tickets/purchase invalid quantity, GET /tickets history) in `services/ticket/tests/integration/handler_test.go`
- [x] T041 [P] [US3] Write unit tests for EmailStatus repository (Create, UpdateStatus, FindPendingRetries, status state transitions) in `services/email/tests/unit/email_repo_test.go`
- [x] T042 [P] [US3] Write unit tests for Kafka consumer (message deserialization, idempotency via ticket_id dedup, commit offset, skip on duplicate) in `services/email/tests/unit/consumer_test.go`
- [x] T043 [P] [US3] Write unit tests for email sender (generate confirmation email content, provider interface mock, send success, send failure with retry increment) in `services/email/tests/unit/sender_test.go`

### Implementation for User Story 3 — Ticket Service

- [x] T044 [P] [US3] Create Ticket model struct (ID, BookingRef, UserID, EventID, Quantity, Status, CreatedAt) in `services/ticket/internal/model/ticket.go`
- [x] T045 [US3] Implement Ticket repository (Create with booking_ref generation/collision retry, FindByUserID with pagination, FindByBookingRef) in `services/ticket/internal/repository/ticket_repo.go`
- [x] T046 [US3] Implement Redis distributed lock (Acquire with key+TTL, Release with Lua script for safe release, auto-expire on TTL) in `services/ticket/internal/lock/redis_lock.go`
- [x] T047 [US3] Implement purchase service (Acquire lock on event → read event remaining_count + date → validate not past, qty>0, qty<=remaining → atomic UPDATE remaining_count = remaining_count - qty → INSERT ticket → publish ticket.purchased event → release lock) in `services/ticket/internal/service/purchase_service.go`
- [x] T048 [US3] Implement Kafka publisher for `ticket.purchased` event (envelope with booking_ref, user_id, event details, quantity, correlation ID) in `services/ticket/internal/publisher/publisher.go`
- [x] T049 [US3] Implement Ticket Service HTTP handlers (POST /tickets/purchase with auth, GET /tickets history with pagination) in `services/ticket/internal/handler/handler.go`
- [x] T050 [US3] Create Ticket Service entry point (wire DB, Redis, Kafka producer, router with auth + logging + correlation middleware, start HTTP on port 8083) in `services/ticket/cmd/main.go`
- [x] T051 [US3] Add Ticket Service to docker-compose.yml with environment variables, port 8083, depends_on MySQL/Redis/Kafka, and health check

### Implementation for User Story 3 — Email Service

- [x] T052 [P] [US3] Create EmailStatus model struct (ID, TicketID, UserID, RecipientHash, Status, RetryCount, LastAttemptAt, CreatedAt) in `services/email/internal/model/email_status.go`
- [x] T053 [US3] Implement EmailStatus repository (Create pending record, UpdateStatus with state transition validation, FindPendingRetries ordered by last_attempt_at) in `services/email/internal/repository/email_repo.go`
- [x] T054 [US3] Implement Kafka consumer for `ticket.purchased` (consume message, check email_status for duplicate ticket_id → if duplicate skip and commit offset → fetch user email from user_db → create email_status record → attempt send → update status to sent/failed → commit offset) in `services/email/internal/consumer/consumer.go`
- [x] T055 [US3] Implement pluggable email sender (EmailProvider interface with Send method, LogProvider for local dev that logs to stdout, SMTP/SendGrid placeholder for production) in `services/email/internal/sender/sender.go`
- [x] T056 [US3] Implement retry scheduler (background goroutine that polls email_status for pending/failed records, applies exponential backoff: 1m→5m→15m→1h→4h, marks dead after 5 retries) in `services/email/internal/sender/retry.go`
- [x] T057 [US3] Create Email Service entry point (wire DB, Kafka consumer, email provider, retry scheduler, start HTTP health endpoint on port 8084) in `services/email/cmd/main.go`
- [x] T058 [US3] Add Email Service to docker-compose.yml with environment variables, port 8084, depends_on MySQL/Kafka, and health check

### Implementation for User Story 3 — Concurrency Stress Test

- [x] T059 [US3] Write concurrency integration test (seed event with 1 ticket → spawn 100 goroutines purchasing simultaneously → assert exactly 1 returns 201, >=99 return 409/423 → assert remaining_count in DB is 0 → assert tickets table has 1 row) in `services/ticket/tests/integration/concurrency_test.go`

**Checkpoint**: User Story 3 is fully functional — concurrent purchase with zero double-sells, async confirmation emails via Kafka, purchase history, stress test passing.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, end-to-end validation, and developer experience improvements

- [x] T060 [P] Write end-to-end integration test (register user → login → browse events → verify email_status) in `services/e2e/e2e_test.go`
- [x] T061 [P] Finalize `Makefile` with all targets: `up` (docker compose up -d), `down`, `build`, `test-unit`, `test-integration`, `test-concurrency`, `test-e2e`, `seed`, `logs-%`, `clean`
- [x] T062 Finalize `docker-compose.yml` with all 4 services, proper `depends_on` with `condition: service_healthy`, restart policies, volume mounts for hot-reload, and network configuration

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion (Go module, Makefile) — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 2 (Phase 4)**: Depends on Foundational completion (independent of US1)
- **User Story 3 (Phase 5)**: Depends on Foundational + US1 (needs auth middleware, user DB)
  - Ticket Service sub-phase: depends on Redis lock, Kafka producer
  - Email Service sub-phase: depends on Kafka consumer, email sender
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) — Depends on US1's auth middleware for session validation; reads user data from user_db

### Within Each User Story

- Tests MUST be written and FAIL before implementation (Constitution IV)
- Models before repositories
- Repositories (data access) before services
- Services before HTTP handlers
- HTTP handlers before main.go entry point
- Core implementation before integration tests
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel (T002, T003, T005, T006, T007, T008)
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- US1 and US2 can be implemented in parallel by different developers after Foundational completes
- All test tasks within a story marked [P] can run in parallel (write all tests first)
- All model tasks marked [P] within a story can run in parallel
- Ticket Service and Email Service sub-phases in US3 can start in parallel after Ticket model/repo

---

## Parallel Example: User Story 3

```bash
# Launch all tests for User Story 3 together (TDD — write tests first):
Task: "Write unit tests for Ticket repository in services/ticket/tests/unit/ticket_repo_test.go"
Task: "Write unit tests for Redis distributed lock in services/ticket/tests/unit/redis_lock_test.go"
Task: "Write unit tests for purchase service in services/ticket/tests/unit/purchase_service_test.go"
Task: "Write unit tests for EmailStatus repository in services/email/tests/unit/email_repo_test.go"
Task: "Write unit tests for Kafka consumer in services/email/tests/unit/consumer_test.go"
Task: "Write unit tests for email sender in services/email/tests/unit/sender_test.go"

# After tests written, launch parallel model + repository tasks:
Task: "Create Ticket model in services/ticket/internal/model/ticket.go"
Task: "Create EmailStatus model in services/email/internal/model/email_status.go"

# Then launch parallel implementation tracks:
# Track A: Ticket Service
Task: "Implement Ticket repository in services/ticket/internal/repository/ticket_repo.go"
Task: "Implement Redis distributed lock in services/ticket/internal/lock/redis_lock.go"
# ... (sequential within track)
# Track B: Email Service
Task: "Implement EmailStatus repository in services/email/internal/repository/email_repo.go"
# ... (sequential within track)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (Registration)
4. **STOP and VALIDATE**: Test registration, login, logout, encrypted PII independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP: user identity system)
3. Add User Story 2 → Test independently → Deploy/Demo (event catalog added)
4. Add User Story 3 → Test independently → Deploy/Demo (full ticket platform)
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (User Service)
   - Developer B: User Story 2 (Event Service)
3. After US1 and US2 complete:
   - Developer A: Ticket Service (US3)
   - Developer B: Email Service (US3)
4. Stories complete and integrate independently

---

## Notes

---

## Phase 7: Convergence

**Purpose**: Address gaps between spec/plan/constitution and current implementation found by convergence audit.

### CRITICAL

- [x] T063 [P] Remove hardcoded encryption key fallback in all service `cmd/main.go` files; crash at startup if ENCRYPTION_KEY is unset per Constitution I / FR-002
- [x] T064 [P] Fix email consumer dedup to use `booking_ref` instead of `event_id` in `services/email/internal/consumer/consumer.go`; add `booking_ref` field to `email_status` table and `scripts/db/init.sql` per FR-011
- [x] T065 [P] Log Kafka publish errors instead of silently discarding them in `services/ticket/internal/service/purchase_service.go` per FR-011/FR-016
- [x] T066 Implement account deletion endpoint (DELETE /api/v1/users) with PII anonymization per FR-020 / Singapore PDPA: replace name_enc/email_enc with redacted values, replace email_hash with random value, preserve tickets with anonymized user ref
- [x] T067 Implement rate limiting middleware for auth endpoints (login 5/min, register 3/min per IP) using Redis counters in `services/shared/pkg/middleware/ratelimit.go` per spec clarifications / FR-005
- [x] T068 Create E2E test covering register → login → browse → purchase → verify email_status in `tests/e2e/flow_test.go` per Constitution IV / T060

### HIGH

- [x] T069 Add `WHERE date > NOW()` to Event repository FindUpcoming query in `services/event/internal/repository/event_repo_mysql.go` per FR-006
- [x] T070 Enrich purchase history with event details (name, date, venue) from event_db in `services/ticket/internal/handler/handler.go` per FR-014
- [x] T071 Fire Kafka publish in a goroutine (async, non-blocking) in `services/ticket/internal/service/purchase_service.go` per SC-005
- [x] T072 Wire crypto package into Email Service `cmd/main.go` and decrypt user email from user_db in `services/email/internal/consumer/consumer.go` per FR-011
- [x] T073 Create email consumer unit tests in `services/email/tests/unit/consumer_test.go` covering deserialization, idempotency, duplicate skip, lookup failure per T042
- [x] T074 Wire `UserEventPublisher` into User Service `cmd/main.go` and call `PublishUserCreated` after successful registration in `services/user/internal/service/auth_service.go` per contracts/kafka-topics.md
- [x] T075 Change Event Service default SERVICE_PORT from 8080 to 8082 in `services/event/cmd/main.go` per plan.md
- [x] T076 Add session TTL refresh in auth middleware after successful token validation in `services/shared/pkg/middleware/auth.go` per US1
- [x] T077 Change bcrypt cost from `bcrypt.DefaultCost` (10) to 12 in `services/user/internal/service/auth_service.go` per data-model.md

### MEDIUM

- [x] T078 Fix `go.mod` Go version from 1.26.3 to 1.22 per plan.md
- [x] T079 Remove unused `sessions` table from `scripts/db/init.sql` (sessions stored in Redis) per data-model.md
- [x] T080 Change lock contention HTTP response from 409 to 423 (StatusLocked) in `services/ticket/internal/handler/handler.go` per contracts/ticket-service.md
- [x] T081 Nest pagination fields under a `"pagination"` key in Event List and Ticket History responses per contracts/event-service.md and ticket-service.md
- [x] T082 Remove CREATE TABLE IF NOT EXISTS from `scripts/seed/seed_events.go`; rely on `init.sql` schema only per data-model.md
- [x] T083 Document cross-DB read tech debt in `services/ticket/cmd/main.go` with TODO comment per plan.md
- [x] T084 Verify Kafka library builds with CGO_ENABLED=0 in `docker/Dockerfile`; add `librdkafka-dev` if needed per plan.md
- [x] T085 Create missing email consumer test file for `HandleTicketPurchased` in `services/email/internal/consumer/consumer_test.go` per Constitution IV

### LOW

- [x] T086 Add distinct "sold out" error response (remaining_count==0) before the generic insufficient check in `services/ticket/internal/handler/handler.go` per FR-008
- [x] T087 Accept `*redis.Client` in `middleware.NewAuth` instead of creating a new client from address string in `services/shared/pkg/middleware/auth.go`
- [x] T088 Fix lock test TTL from nanoseconds to seconds in `services/ticket/tests/integration/integration_test.go` per test quality
- [x] T089 Document session invalidation strategy for future password change feature in `services/user/internal/service/auth_service.go` per spec edge cases
- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story must be independently completable and testable
- Constitution IV (TDD): Write tests first, verify they FAIL, then implement
- Constitution I: Verify PII encryption on every registration (unit test + manual MySQL inspection)
- Constitution II: Concurrency stress test (T059) must pass before US3 is considered complete
- Constitution III: Email confirmation must be fully async — purchase handler must not await email send
- Commit after each task or logical group of tasks
- Stop at any checkpoint to validate story independently
