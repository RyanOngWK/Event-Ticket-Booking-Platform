---
title: "User Service"
category: "entity"
tags: [service, authentication, pii, encryption, sessions]
source_count: 7
updated: 2025-07-16
---

# User Service

## Overview

The User Service (`:8081`) owns identity, authentication, and PII management. It is the sole source of truth for user accounts. All PII (name, email) is encrypted at the application layer using AES-256-GCM before writing to the database.

## Responsibilities

- User registration (rate limited: 3/min per IP)
- Login / logout (session token creation and invalidation)
- Profile retrieval (`GET /me`)
- Account deletion with PII anonymization (Singapore PDPA compliant)
- PII encryption/decryption at the application layer

## Interfaces

### REST API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/users/register` | — | Create account |
| `POST` | `/api/v1/users/login` | — | Authenticate, returns session token |
| `POST` | `/api/v1/users/logout` | Bearer | Invalidate current session |
| `POST` | `/api/v1/users/delete` | Bearer | Anonymize PII, preserve ticket records |
| `GET` | `/api/v1/users/me` | Bearer | Return authenticated profile |

### Kafka

Publishes to `user.created` on registration. Enables future services (welcome emails, analytics) to react to new accounts.

## Data Model

Database: `user_db`. Table: `users`.

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT |
| `name_enc` | VARBINARY(512) | AES-256-GCM encrypted |
| `email_enc` | VARBINARY(512) | AES-256-GCM encrypted |
| `email_hash` | VARCHAR(64) | SHA-256 hash, UNIQUE, for duplicate detection |
| `password_hash` | VARCHAR(255) | bcrypt cost factor 12 |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | ON UPDATE |

Full schema: [data-model.md](../specs/001-event-ticket-booking/data-model.md#user-service-database-user_db)

## Key Decisions

- **Opaque session tokens over JWTs** — tokens are stored in [[redis]], not self-contained. Enables instant revocation on logout without a blocklist. See [[session-management]].
- **Per-user encryption keys** derived from a master key — limits blast radius if a key is compromised. See [[pii-encryption]].
- **bcrypt cost factor 12** for password hashing.

## Cross-references

- [[session-management]] — how authentication tokens work
- [[pii-encryption]] — encryption strategy for PII fields
- [[redis]] — session store
- [[mysql]] — user_db schema
- [[kafka]] — `user.created` topic
- [[constitution]] — Principle I (Security-First)
- [[event-service]] — companion public-facing service
- [[sources/specs]] — traceability to spec requirements
- [[sources/code-structure]] — source location: `services/user/`
