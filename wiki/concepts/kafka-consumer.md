---
title: "Kafka Consumer (Code Walkthrough)"
category: "concept"
tags: [kafka, consumer, messaging, go, code-walkthrough, idempotency]
source_count: 4
updated: 2026-08-12
---

# Kafka Consumer (Code Walkthrough)

## Overview

The consumer is the **reading side** of [[kafka]]. It subscribes to topics, waits for messages, and hands each one to the right handler. In this codebase there is exactly one consumer service: [[email-service]], which reads `ticket.purchased` events and sends confirmation emails.

Like the producer, there are **two layers**:

1. A **shared `Consumer`** in `services/shared/pkg/kafka/consumer.go` — the generic subscribe-and-read loop.
2. A **service handler** in `services/email/internal/consumer/consumer.go` — `EmailConsumer.HandleTicketPurchased`, which does the actual work when a purchase event arrives.

The flow: `Kafka topic → Consumer.Consume() loop → handler by topic → email logic`.

## Shared Consumer Walkthrough

### Creating the consumer

```go
c, err := kafka.NewConsumer(brokers, groupID, topics)  // consumer.go:17
```

Config settings (`consumer.go:18`):

| Setting | Value | Why |
|---------|-------|-----|
| `bootstrap.servers` | the broker address | Where to connect |
| `group.id` | the consumer group | Multiple consumers in a group **share** the topic's partitions; each message goes to exactly one of them |
| `auto.offset.reset` | `earliest` | On first start (no committed offset), read from the beginning of the topic |
| `enable.auto.commit` | `false` | Offsets are committed **manually**, only after a message is fully processed (see below) |

### Registering handlers

```go
c.RegisterHandler("ticket.purchased", handler)  // consumer.go:39
```

Stores the handler in a map keyed by topic name. When a message arrives on `ticket.purchased`, this is the function that gets called.

### The Consume loop

```go
func (c *Consumer) Consume() error  // consumer.go:43
```

This is an **infinite loop** (`for {}` — runs forever until the process stops). Each iteration:

1. **`ReadMessage(-1)`** (`consumer.go:45`) — blocks until the next message arrives (`-1` = wait indefinitely). Returns an error if the connection breaks.
2. **Look up the handler** by topic (`consumer.go:50`) — if no handler is registered for that topic, the message is logged and skipped.
3. **Unmarshal** the JSON bytes into an `EventEnvelope` (`consumer.go:57`) — the reverse of what `Producer.Publish` did (see [[kafka-producer]]).
4. **Call the handler** (`consumer.go:63`) — `handler(&envelope, msg)`. If it returns an error, the message is logged and the loop continues (the offset is NOT committed).
5. **Commit the offset** (`consumer.go:68`) — `CommitMessage(msg)` tells Kafka "I finished this message, don't redeliver it."

This **commit-after-success** pattern is what gives **at-least-once delivery**: if the handler crashes before committing, the message is redelivered — which is why the handler must be **idempotent** (safe to process twice). See [[service-decoupling]].

Go note: `Consumer` methods use a plain receiver (`c *Consumer`) and `handlers` is a `map[string]HandlerFunc` — a map from topic name to a function. `HandlerFunc` is defined as `func(envelope *EventEnvelope, msg *kafka.Message) error` (`consumer.go:10`) — a **function type**, i.e. a variable that holds a function.

## Email Consumer Handler

### The struct and its dependencies

```go
type EmailConsumer struct {
    repo          repository.EmailRepository
    userDB        *sql.DB
    emailProvider sender.EmailProvider
    crypto        *crypto.Crypto
    userEmailFunc func(userID uint64) (string, error)  // consumer.go:33
}
```

This is **dependency injection**: the handler doesn't create its own database/email connections — they're passed in (via `New(...)`, `consumer.go:36`). That's what makes it testable (see below).

`userEmailFunc` is a special field: a **function stored as a value**. In production it's `nil` (the handler falls back to a real SQL query). In tests it's overridden with a stub — see the Go note at the end.

### HandleTicketPurchased, step by step

```go
func (c *EmailConsumer) HandleTicketPurchased(envelope *sharedkafka.EventEnvelope, msg *kafkago.Message) error  // consumer.go:45
```

1. **Extract the payload** (`consumer.go:46`) — re-marshal `envelope.Payload` to JSON, then unmarshal into the typed `TicketPurchasePayload` struct. (The envelope's `Payload` field is `interface{}`, so it's re-typed here.)
2. **Dedupe check** (`consumer.go:56`) — `repo.FindByBookingRef(payload.BookingRef)`. If a status row already exists for this `booking_ref`, log and return early. This is the **idempotency** safeguard against at-least-once redelivery: the same purchase never sends two emails.
3. **Fetch the buyer's email** (`consumer.go:65`) — `fetchUserEmail(userID)`:
   - Uses `userEmailFunc` if set (tests), otherwise queries `user_db` for the encrypted email and decrypts it (`consumer.go:113`). Emails are stored encrypted — see [[pii-encryption]].
   - Returns an error if the user doesn't exist.
4. **Create a `pending` status row** (`consumer.go:70`) — insert into `email_db.email_status` with `Status: "pending"`, `RetryCount: 0`. This row is what the retry/dead-letter machinery acts on — see [[email-retry-strategy]].
5. **Send the email** (`consumer.go:82`) — `sender.SendConfirmationEmail(...)` through the pluggable `EmailProvider`.
   - **On success**: update the row to `sent` (`consumer.go:101`), return `nil` → the shared loop commits the offset.
   - **On failure**: log, update the row to `failed` with `RetryCount: 1` (`consumer.go:95`), and still return `nil`. Returning `nil` is deliberate — the failure is recorded in the database for retry rather than blocking the consumer loop. (Note: retry scheduling beyond recording the failure is future scope — see [[email-retry-strategy]].)

Go note: `userEmailFunc` being a field means tests can **swap the function without a database**:

```go
consumer.userEmailFunc = func(userID uint64) (string, error) {
    return "user@example.com", nil
}
```

This is the seam that lets the unit tests run without MySQL (see [[testing-strategy]]).

## How the Consumer Is Tested

The test file is `services/email/internal/consumer/consumer_test.go`. Tests do **not** run Kafka at all — they:

1. Build an `EmailConsumer` with a **mock repo** (in-memory maps) and a **mock email provider** (`consumer_test.go:91`).
2. Construct a `kafkago.Message` struct by hand with a JSON payload (`consumer_test.go:135`).
3. Call `consumer.HandleTicketPurchased(payload, msg)` **directly** — bypassing the shared `Consume()` loop entirely.

So the tests verify the **handler logic** (dedupe, send, failed-status) but not topic delivery, offsets, or broker behavior — that's the known fidelity gap in [[testing-strategy]].

## Cross-references

- [[kafka]] — the async backbone, topics, and message format
- [[kafka-producer]] — the matching sending side
- [[email-service]] — the service that owns this consumer
- [[service-decoupling]] — why consumers exist and the at-least-once/idempotency contract
- [[email-retry-strategy]] — retries and dead-lettering after send failures
- [[pii-encryption]] — email decryption at read time
- [[testing-strategy]] — how consumer logic is tested without a broker
- [[sources/code-structure]] — source location: `services/shared/pkg/kafka/consumer.go` and `services/email/internal/consumer/consumer.go`
