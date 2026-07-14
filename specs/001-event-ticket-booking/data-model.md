# Data Model: Event Ticket Booking Platform

**Date**: 2025-07-14

## Entity Relationship

```
┌──────────────┐       ┌──────────────────┐       ┌──────────────┐
│    User      │       │      Event       │       │    Ticket    │
├──────────────┤       ├──────────────────┤       ├──────────────┤
│ id (PK)      │       │ id (PK)          │       │ id (PK)      │
│ name_enc     │       │ name             │       │ booking_ref  │
│ email_hash   │       │ description      │       │ user_id (FK) │
│ email_enc    │       │ date             │       │ event_id(FK) │
│ password_hash│       │ venue            │       │ quantity     │
│ created_at   │       │ total_capacity   │       │ status       │
│ updated_at   │       │ remaining_count  │       │ created_at   │
└──────┬───────┘       │ created_at       │       └──────────────┘
       │               │ updated_at       │
       │               └──────────────────┘
       │
       │   ┌──────────────────┐
       └───│   EmailStatus    │
           ├──────────────────┤
           │ id (PK)          │
           │ ticket_id (FK)   │
           │ user_id (FK)     │
           │ recipient_hash   │
           │ status           │
           │ retry_count      │
           │ last_attempt_at  │
           │ created_at       │
           └──────────────────┘
```

## Entities

### User Service Database (`user_db`)

**Table: `users`**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT | Unique user identifier |
| `name_enc` | VARBINARY(512) | NOT NULL | AES-256-GCM encrypted name |
| `email_enc` | VARBINARY(512) | NOT NULL | AES-256-GCM encrypted email |
| `email_hash` | VARCHAR(64) | UNIQUE, NOT NULL | SHA-256 hash of email for uniqueness lookups and duplicate detection |
| `password_hash` | VARCHAR(255) | NOT NULL | bcrypt hash of password |
| `created_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Account creation time |
| `updated_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | Last update time |

**Indexes**: UNIQUE on `email_hash`; INDEX on `created_at`.

**Validation Rules**:
- `email_hash`: derived from lowercase(trim(email)) → SHA-256
- `password_hash`: bcrypt cost factor 12
- `name_enc`: non-empty before encryption, max 255 chars plaintext
- `email_enc`: valid email format before encryption

### Event Service Database (`event_db`)

**Table: `events`**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT | Unique event identifier |
| `name` | VARCHAR(255) | NOT NULL | Event name |
| `description` | TEXT | NOT NULL | Event description |
| `date` | DATETIME | NOT NULL | Event date/time |
| `venue` | VARCHAR(255) | NOT NULL | Event venue/location |
| `total_capacity` | INT UNSIGNED | NOT NULL | Total tickets available for this event |
| `remaining_count` | INT UNSIGNED | NOT NULL | Tickets still available |
| `created_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | Last update time |

**Constraints**: CHECK `remaining_count >= 0` AND `remaining_count <= total_capacity`.

**Indexes**: INDEX on `date`; INDEX on `remaining_count` (for sold-out filtering).

**Validation Rules**:
- `date`: must be in the future for active events
- `total_capacity`: > 0
- `remaining_count`: >= 0, initially equal to `total_capacity`

### Ticket Service Database (`ticket_db`)

**Table: `tickets`**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT | Internal ticket ID |
| `booking_ref` | CHAR(12) | UNIQUE, NOT NULL | Public booking reference (e.g., "TBK-A3F8X2K1") |
| `user_id` | BIGINT UNSIGNED | FK → `user_db.users.id`, NOT NULL | Purchasing user |
| `event_id` | BIGINT UNSIGNED | FK → `event_db.events.id`, NOT NULL | Event purchased for |
| `quantity` | INT UNSIGNED | NOT NULL | Number of tickets purchased |
| `status` | ENUM('confirmed', 'cancelled') | NOT NULL, DEFAULT 'confirmed' | Ticket status |
| `created_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Purchase time |

**Indexes**: UNIQUE on `booking_ref`; INDEX on `user_id`; INDEX on `event_id`; INDEX on `created_at`.

**Validation Rules**:
- `booking_ref`: generated as `TBK-` + 8 random alphanumeric chars (uppercase), collision retry on insert
- `quantity`: > 0
- `status`: 'confirmed' on creation, 'cancelled' if purchase is rolled back

### Email Service Database (`email_db`)

**Table: `email_status`**

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT | Internal status ID |
| `ticket_id` | BIGINT UNSIGNED | FK → `ticket_db.tickets.id`, NOT NULL | Associated ticket |
| `user_id` | BIGINT UNSIGNED | FK → `user_db.users.id`, NOT NULL | Recipient user |
| `recipient_hash` | VARCHAR(64) | NOT NULL | SHA-256 hash of recipient email for lookup |
| `status` | ENUM('pending', 'sent', 'failed', 'dead') | NOT NULL, DEFAULT 'pending' | Delivery status |
| `retry_count` | INT UNSIGNED | NOT NULL, DEFAULT 0 | Number of delivery retries |
| `last_attempt_at` | TIMESTAMP | NULL | Timestamp of last delivery attempt |
| `created_at` | TIMESTAMP | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Record creation time |

**Indexes**: INDEX on `ticket_id`; INDEX on `status`; INDEX on `last_attempt_at` (for retry scheduling).

**State Transitions**:
```
pending → sent       (successful delivery)
pending → failed     (delivery attempt failed, retry scheduled)
failed  → pending   (retry initiated)
failed  → dead       (max retries exceeded, 5 attempts)
pending → dead       (max age exceeded, 24h without success)
```

**Validation Rules**:
- `status`: only valid transitions allowed
- `retry_count`: increments on each failed attempt, max 5 before `dead`

## Cross-Service Data Access

| Reader | Data Needed | Access Method |
|--------|-------------|---------------|
| Ticket Service | Event `remaining_count` | Direct read from `event_db.events` (shared MySQL instance) or Event Service API |
| Ticket Service | Event `date` (past-event check) | Direct read from `event_db.events` |
| Email Service | User email (to send to) | Direct read from `user_db.users.email_enc` with decryption key |
| Ticket Service | User existence validation | Direct read from `user_db.users.id` |

For v1, services running in the same Docker network with access to all MySQL databases is
acceptable for simplicity. In future iterations, cross-service reads should go through service
APIs.
