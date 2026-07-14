# Kafka Topic Contracts

## Topics

### `user.created`

Emitted when a new user registers. Consumed by future services (welcome emails, analytics).

**Key**: `user_id` (string)

**Value**:
```json
{
  "event_id": "evt_a1b2c3d4",
  "event_type": "user.created",
  "timestamp": "2025-07-14T10:00:00Z",
  "correlation_id": "corr_x1y2z3",
  "payload": {
    "user_id": 42,
    "email_hash": "sha256:abc123...",
    "created_at": "2025-07-14T10:00:00Z"
  }
}
```

---

### `ticket.purchased`

Emitted when a ticket purchase is confirmed. Consumed by the Email Service for sending
confirmation emails.

**Key**: `booking_ref` (string)

**Value**:
```json
{
  "event_id": "evt_f5g6h7i8",
  "event_type": "ticket.purchased",
  "timestamp": "2025-07-14T14:30:00Z",
  "correlation_id": "corr_j9k0l1m2",
  "payload": {
    "booking_ref": "TBK-A3F8X2K1",
    "user_id": 42,
    "event_id": 1,
    "event_name": "Summer Music Festival",
    "event_date": "2025-08-15T18:00:00Z",
    "venue": "Central Park Amphitheater",
    "quantity": 2,
    "purchased_at": "2025-07-14T14:30:00Z"
  }
}
```

---

## Message Envelope

All Kafka messages share a common envelope:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event_id` | string (UUID v4) | yes | Unique identifier for this event instance (idempotency) |
| `event_type` | string | yes | Reverse-DNS event type (`user.created`, `ticket.purchased`) |
| `timestamp` | string (ISO 8601) | yes | When the event was generated |
| `correlation_id` | string | yes | Request trace ID for end-to-end debugging |
| `payload` | object | yes | Event-specific data (see per-topic schemas above) |

## Consumer Contracts

### Email Service

- **Subscribes to**: `ticket.purchased`
- **Consumer Group**: `email-service`
- **Processing**: Idempotent — checks `email_status` table by `ticket_id` before sending.
  Commits offset only after successful MySQL write of email status.
- **Retry**: Exponential backoff (1m → 5m → 15m → 1h → 4h → dead-letter after 5 failures).
  Failed messages are NOT committed; consumer pauses and retries on next poll.
- **Dead Letter**: After 5 failures, status set to `dead` in MySQL. Offset committed to
  prevent blocking the partition.
