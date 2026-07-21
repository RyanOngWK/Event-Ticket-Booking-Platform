# PROGRESS.md

> Updated: 2026-07-21 · Session state tracker for LLM-driven development.

---

## Status Snapshot

| Area | Status |
|------|--------|
| **Phase 1–7 (89 tasks)** | ✅ Complete |
| **Constitution compliance** | ✅ All 4 principles verified |
| **Tests** | ✅ 112 test functions (87 unit + 24 integration + 1 e2e) |
| **Wiki** | ✅ |

---

## Done

- [x] **Phase 1: Setup** (T001–T008) — Docker infrastructure, Go module, Makefile, Kafka bootstrap
- [x] **Phase 2: Foundational** (T009–T016) — Crypto, Kafka, middleware, auth packages
- [x] **Phase 3: US1 — User Registration** (T017–T026) — Encrypted PII, session auth, `user.created` Kafka events
- [x] **Phase 4: US2 — Event Browsing** (T027–T036) — Public catalog, pagination, sold-out indicators, seed data
- [x] **Phase 5: US3 — Ticket Purchase** (T037–T059) — Distributed locking, concurrent purchase, async email via Kafka
- [x] **Phase 6: Polish** (T060–T062) — E2E test, Makefile finalization, docker-compose hardening
- [x] **Phase 7: Convergence** (T063–T089) — 27 audit gaps resolved: encryption key enforcement, dedup fix, rate limiting, PII anonymization, TDD gaps filled
- [x] **Wiki initialized** — `wiki/SCHEMA.md`, `wiki/index.md`, initial source ingestion

Full task list: [`specs/001-event-ticket-booking/tasks.md`](specs/001-event-ticket-booking/tasks.md)

---

## In Progress

> *Nothing currently in progress. Project is in a stable, completed v1 state.*

---

## Under Consideration

*Features from the [deferred list](README.md#whats-deferred-future-scope) — ranked by likely priority:*

| Rank | Feature | Effort | Rationale |
|------|---------|--------|-----------|
| 1 | **Payment processing** | Large | Separates inventory from financial concern; adds PCI compliance scope |
| 2 | Real email provider (SendGrid/SMTP) | Small | Provider interface already pluggable; just swap `LogProvider` |
| 3 | Production monitoring (Prometheus/Grafana) | Medium | Structured logging with correlation IDs already in place |
| 4 | gRPC service-to-service communication | Medium | Cross-DB reads currently work; proper APIs for event checks |
| 5 | Event management admin CRUD | Medium | Currently seed-data only; no admin API to create/update events |
| 6 | Kubernetes manifests / Helm charts | Medium | Compose works for 4-service topology; K8s for prod deployment |
| 7 | OAuth / social login | Medium | Email+password works; third-party auth is orthogonal to ticket flow |
| 8 | Event search/filtering (keyword, date range, venue) | Medium | Premature without production traffic patterns |
| 9 | Multi-factor auth (TOTP/SMS) | Medium | Adds infrastructure without changing core ticket flow |
| 10 | Multi-region deployment | Large | Cross-region Redis lock coordination adds significant complexity |

A decision on which feature to tackle next has not been made. Spec-driven workflow entry point for new features:
1. Define feature spec in `specs/002-<feature-name>/spec.md`
2. Run `plan` → `tasks` → `implement` → `converge` per [`AGENTS.md`](AGENTS.md) and constitution

---

## Blocked

> *No items currently blocked.*

---

## Links

- [AGENTS.md](AGENTS.md) — LLM wiki conventions and workflow
- [README.md](README.md) — Project overview, architecture, API reference
- [`specs/001-event-ticket-booking/tasks.md`](specs/001-event-ticket-booking/tasks.md) — Full task breakdown
- [`.specify/memory/constitution.md`](.specify/memory/constitution.md) — 4 non-negotiable principles
- [`.opencode/commands/`](.opencode/commands/) — Available agent commands (speckit.*)
