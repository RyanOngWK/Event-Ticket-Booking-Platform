# Quickstart: Event Ticket Booking Platform

**Date**: 2025-07-14

## Prerequisites

- Docker and Docker Compose v2+
- Go 1.22+ (for running tests outside Docker)
- `make` (for convenience targets)

## Setup

### 1. Start Infrastructure & Services

```bash
docker compose up -d
```

This starts: MySQL (port 3306), Redis (port 6379), Kafka + Zookeeper (ports 9092/2181),
User Service (port 8081), Event Service (port 8082), Ticket Service (port 8083),
Email Service (port 8084).

Verify all services are healthy:

```bash
docker compose ps
```

All services should show `healthy` status.

### 2. Seed Event Data

```bash
make seed
```

This inserts sample events into MySQL (Event Service database).

### 3. Run Tests

```bash
# All unit tests
make test-unit

# All integration tests
make test-integration

# Concurrency stress test
make test-concurrency
```

## Validation Scenarios

### Scenario 1: User Registration & Login

```bash
# Register a user
curl -X POST http://localhost:8081/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane Doe","email":"jane@example.com","password":"secure123"}'
# Expected: 201 Created with user_id

# Attempt duplicate registration
curl -X POST http://localhost:8081/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane Doe","email":"jane@example.com","password":"secure123"}'
# Expected: 409 Conflict "An account with this email already exists"

# Login
TOKEN=$(curl -s -X POST http://localhost:8081/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"jane@example.com","password":"secure123"}' | jq -r '.token')
# Expected: returns a session token

# Verify encrypted storage (inspect MySQL directly)
docker compose exec mysql mysql -u root -e \
  "SELECT email_enc FROM user_db.users WHERE id=1;"
# Expected: binary data (not plaintext email)
```

### Scenario 2: Browse Events (Public)

```bash
# List all events (no auth required)
curl http://localhost:8082/api/v1/events
# Expected: 200 OK with array of events, each showing name, date, venue, remaining_count

# Get event details
curl http://localhost:8082/api/v1/events/1
# Expected: 200 OK with full event details including description, total_capacity

# Sold-out event shows correctly
curl http://localhost:8082/api/v1/events/1 | jq '.sold_out'
# Expected: false (for events with remaining_count > 0)
```

### Scenario 3: Concurrent Ticket Purchase

```bash
# First, check remaining tickets for event 1
REMAINING=$(curl -s http://localhost:8082/api/v1/events/1 | jq '.remaining_count')
echo "Remaining: $REMAINING"

# Purchase tickets as Jane
curl -X POST http://localhost:8083/api/v1/tickets/purchase \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"event_id":1,"quantity":2}'
# Expected: 201 Created with booking_ref like "TBK-XXXXXXXX"

# Verify remaining count decreased
curl http://localhost:8082/api/v1/events/1 | jq '.remaining_count'
# Expected: $REMAINING - 2

# Attempt to buy more than available
curl -X POST http://localhost:8083/api/v1/tickets/purchase \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"event_id":1,"quantity":99999}'
# Expected: 409 Conflict with "Not enough tickets available"

# View purchase history
curl http://localhost:8083/api/v1/tickets \
  -H "Authorization: Bearer $TOKEN"
# Expected: 200 OK with list of purchased tickets
```

### Scenario 4: Concurrency Stress Test

```bash
# Requires `hey` or `wrk` load testing tool
# Creates an event with exactly 1 ticket, then fires 100 concurrent purchases

make test-concurrency
```

**Expected outcome**: Exactly 1 purchase succeeds (201), 99 are declined (409 or 423).
Zero double-sells — verified by checking that the event's `remaining_count` is 0 and
the tickets table has exactly 1 row for that event.

### Scenario 5: Async Email Confirmation

```bash
# After a successful purchase, check Email Service logs
docker compose logs email | grep "confirmation sent"

# Expected: log entry showing email processed from ticket.purchased event

# Check email status in MySQL
docker compose exec mysql mysql -u root -e \
  "SELECT * FROM email_db.email_status ORDER BY created_at DESC LIMIT 1;"
# Expected: row with status='sent', retry_count=0
```

### Scenario 6: PII Encryption Verification

```bash
# Verify that user PII is encrypted at rest
docker compose exec mysql mysql -u root -e \
  "SELECT id, name_enc, email_enc, email_hash FROM user_db.users LIMIT 1;"
# Expected: name_enc and email_enc contain binary (encrypted) data.
# email_hash contains a 64-char hex string (SHA-256).
# No plaintext PII visible.
```

## Troubleshooting

- **Service fails to start**: Run `docker compose logs <service>` to check for errors.
  Common issues: missing env vars, MySQL not ready (wait for health check).
- **Kafka consumer not processing**: Check that topics were auto-created:
  `docker compose exec kafka kafka-topics --list --bootstrap-server localhost:9092`
- **Lock contention errors (HTTP 423)**: Normal under high concurrency. Clients should retry
  with backoff.
