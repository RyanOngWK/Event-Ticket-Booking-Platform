---
title: "PII Encryption"
category: "concept"
tags: [security, encryption, pii, aes-256-gcm, cryptography]
source_count: 7
updated: 2025-07-16
---

# PII Encryption

## Overview

All Personally Identifiable Information (name, email) is encrypted at the application layer using AES-256-GCM before writing to MySQL. Encryption is a MUST principle under the [[constitution]] (Principle I: Security-First).

## Rationale

**Why application-level encryption over MySQL TDE (Transparent Data Encryption):**

- MySQL TDE requires Enterprise Edition — a licensing dependency.
- TDE encrypts at the file level, not the column level. An attacker with database access could still read plaintext columns.
- Application-level encryption gives column-level control: only specific PII fields are encrypted.
- Database-agnostic: switching to PostgreSQL or another store doesn't change the encryption layer.
- Key management is decoupled from the database tier — supports upgrading to AWS KMS / HashiCorp Vault without code changes.

## Implementation

- **Algorithm**: AES-256-GCM (authenticated encryption — provides both confidentiality and integrity)
- **Key derivation**: Per-user encryption keys derived from a master key, limiting blast radius if a single key is compromised
- **Master key**: Stored in environment variable `ENCRYPTION_KEY` (32 bytes base64-encoded), never in source code
- **Hash**: SHA-256 hash of email stored separately for uniqueness lookups without decryption

### Code Location

Shared encryption library: `services/shared/pkg/crypto/`

### Encrypted Fields

| Table | Field | Type |
|-------|-------|------|
| `user_db.users` | `name_enc` | VARBINARY(512) |
| `user_db.users` | `email_enc` | VARBINARY(512) |

### Verification

Direct database inspection confirms no plaintext PII visible:
```bash
docker compose exec mysql mysql -u root -prootpassword -e \
  "SELECT id, name_enc, email_enc, email_hash FROM user_db.users LIMIT 1;"
# → name_enc and email_enc are binary blobs. No plaintext PII visible.
```

## Alternatives Considered

| Alternative | Why Rejected |
|-------------|-------------|
| MySQL TDE | Enterprise Edition only, file-level encryption, not column-level |
| AWS KMS envelope encryption | Deferred to future — adds cloud dependency for v1. Architecture supports upgrade path. |

## Key Constraints (from Constitution)

- "Encryption MUST use industry-standard algorithms (AES-256-GCM or equivalent)"
- "Encryption keys MUST NOT be stored in source code or configuration files"
- "Plaintext PII MUST never appear in logs, error messages, or analytics pipelines"

## Cross-references

- [[constitution]] — Principle I (Security-First)
- [[user-service]] — encrypts/decrypts PII
- [[email-service]] — decrypts email at read time for sending
- [[mysql]] — stores encrypted VARBINARY columns
- [[session-management]] — complementary security concern
- [[sources/code-structure]] — shared crypto package
