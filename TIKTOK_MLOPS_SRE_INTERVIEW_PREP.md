# TikTok MLOps/SRE Interview Prep

This guide uses the Event Ticket Booking Platform as the example system. The answers are intentionally candid: describe what v1 implements, name the failure mode, then explain the production evolution. Do not claim Redlock, exactly-once delivery, complete production isolation, or fully infrastructure-backed end-to-end testing.

## Answer Pattern

Use this structure for deep dives:

> "The specification established [invariant or requirement]. I chose [design] because [constraint]. It provides [benefit]. Its failure mode is [risk], which I would observe with [signals]. At [scale or reliability threshold], I would evolve it to [next design]."

## Spec-Driven Development

### Walk me through one requirement from constitution/specification to production behavior. How did you prove it?

The strongest example is the no-oversell invariant. The constitution made concurrency management mandatory, the functional specification defined the purchase behavior and sold-out case, and the plan/tasks turned that into a Ticket Service flow. The implementation acquires a per-event Redis lease, then makes the final inventory decision with a conditional SQL update: `remaining_count = remaining_count - quantity WHERE remaining_count >= quantity`. A zero-row update means sold out; a successful update cannot make inventory negative. I proved the business invariant with concurrency-oriented tests and code review of the database predicate. For production confidence, I would add a real-MySQL race test, `go test -race`, and load tests that verify successful allocations never exceed initial inventory.

### How did you resolve ambiguity or contradictory requirements in the specification?

I treated the invariant and explicitly stated acceptance criteria as higher priority than an implementation suggestion. For example, a Redis lock can reduce contention, but the requirement is correct inventory allocation, not merely lock acquisition. I made conditional SQL the final authority because it directly enforces non-negative inventory even if the lock layer is unavailable or behaves incorrectly. When documentation and code differ, I describe the implemented behavior and record the delta rather than preserving an inaccurate claim.

### What did implementation reveal that the specification missed, and how did you handle it?

The design described a Redlock-style mechanism and complete retry/DLQ behavior, but the implementation is a single Redis lease and does not fully persist retry state or enforce the advertised terminal retry policy. I would classify these as gaps, not call them completed features. I would update the decision record, create prioritized work items, add regression tests first, and define measurable acceptance criteria such as owner-token-safe release, fencing-token rejection of stale writes, persisted attempt counts, and alerting on dead-letter age.

### How did you keep specifications from becoming stale documentation?

The project keeps a traceability chain: constitution, specification, plan, tasks, implementation, and tests. I would require a pull request to update the relevant specification or decision record whenever it changes a contract, invariant, failure mode, or operational procedure. The practical control is review: code changes should link to a requirement, and behavior-changing production incidents should result in a runbook, test, or specification update. I would also periodically reconcile claimed behaviors against executable tests and deployed configuration.

### Which requirements are executable and which are prose-only?

Executable requirements include API contracts, validation, authentication behavior, pagination, inventory predicates, and test commands in CI. Requirements such as "production-grade," "secure," or "reliable retries" are not executable until decomposed into measurable controls: an SLO, encryption/key-management requirement, unique idempotency constraint, maximum retry age, or chaos test. A key lesson from this project is that broad prose must become an observable acceptance criterion before it can be trusted.

### How did tasks capture dependencies, rollback, and operational acceptance criteria?

The original tasks captured implementation dependencies across services, schemas, contracts, and test phases. For a production MLOps/SRE workflow, I would extend each change with migration safety, dashboard/alert impact, rollback conditions, capacity impact, and owner. For example, an outbox rollout would include a backward-compatible schema migration, a relay metric, a backlog-age alert, a staged enablement flag, and a rollback that stops new publishing without deleting undelivered rows.

### What did spec-driven development improve, and what did it slow down?

It improved traceability, reduced ambiguity in service boundaries and contracts, and made the concurrency invariant explicit before implementation. It also created a useful basis for test planning and trade-off discussions. It slows early iteration when requirements are genuinely unknown and can create false confidence if prose is mistaken for verification. I keep it lightweight for reversible work and use more rigor for data migrations, reliability guarantees, and public interfaces.

## Reliability And Inventory Correctness

### Prove no overselling under concurrency, Redis failure, lease expiry, retries, and partial database failures.

The conditional SQL predicate is the narrow proof for non-negative inventory: a successful update requires sufficient remaining inventory at write time. Redis reduces duplicate concurrent work but is not the proof. If Redis is unavailable, the current service cannot acquire the lease and should fail the request rather than risk an uncontrolled path; I would make that dependency policy explicit and alert on it. Lease expiry creates a zombie-request window in the current implementation, so mutual exclusion is not fully guaranteed. The SQL predicate still protects non-negative count, but it does not protect against all duplicate or partial-work outcomes. Retries require an idempotency key. A decrement followed by failed ticket insertion can consume inventory without a ticket because the writes span databases. The production fix is a transactional reservation/inventory authority, purchase idempotency, and an outbox or saga for downstream state.

### Why is Redis locking insufficient for correctness?

Redis locking is advisory serialization around a shared resource. A lease can expire while the original holder is paused, the holder can lose network connectivity, Redis can fail over, and a stale holder can resume. In this project the lock value is constant, so an old request can even release a newer request's lock after expiry. Correctness must be enforced by the durable system that owns the invariant: here, conditional SQL prevents negative inventory. For stronger protection, I would use per-acquisition ownership tokens for release and monotonic fencing tokens validated by the write authority.

### What happens when a request exceeds the 30-second lock TTL?

Request A can continue after its lease expires. Request B can acquire the same event lock and proceed. If A later releases a lock using the shared constant value, it can delete B's lease; A may also continue performing stale work. The conditional decrement avoids a negative count, but mutual exclusion and efficiency are broken. I would generate a cryptographically random owner token for every acquisition and release only when it matches. To reject stale writes entirely, I would add a monotonically increasing fencing token and have the inventory store reject writes with an older token.

### Why is this not Redlock, and what do fencing tokens protect against?

It is not Redlock because it uses one Redis instance, has no quorum across independent Redis nodes, no Redlock acquisition algorithm, and no fencing tokens. Calling it Redlock would overstate its guarantees. Fencing tokens are ordered lease-generation numbers. The resource owner persists the newest accepted token and rejects writes carrying lower values. That stops a previously paused or partitioned client from modifying the resource after a newer owner acquired the lock.

### The inventory decrement and ticket insert cross databases. What can fail, and how would you recover?

If the Event database decrement succeeds and the Ticket database insert fails, inventory is consumed without a ticket. Retrying without an idempotency boundary can create duplicate tickets or unclear recovery. The current design has neither a distributed transaction nor a compensation record. I would prefer moving inventory and reservation state into one transactional authority. If separate services are necessary, I would create a reservation state machine with expiry, a stable purchase idempotency key, durable saga/outbox records, and reconciliation jobs that detect and repair stranded reservations.

### How would you add payment authorization, reservation expiry, cancellation, and compensation?

I would replace immediate confirmation with an explicit state machine: `pending_payment`, `payment_authorized`, `confirmed`, `expired`, `cancelled`, and `refunded` where applicable. Creating a pending reservation atomically reduces available inventory and writes an outbox event. A payment worker processes an idempotent provider request and confirmation webhook. A timeout worker expires unpaid reservations and releases inventory through an idempotent compensation. Every external callback and transition uses a stable idempotency key, an audit record, and metrics for state age and compensation failures.

### What load test would you run for a flash-sale event?

I would seed a known inventory, for example 1,000 tickets, then issue far more concurrent requests with realistic authentication and retry behavior. I would vary one hot event versus distributed events, increase latency around Redis and MySQL, and inject lease-expiry and retry scenarios. Success criteria are: successful ticket count never exceeds inventory; remaining inventory is never negative; every accepted purchase maps to one durable ticket/reservation; p99 latency and error/lock-conflict rates remain within stated SLOs; and recovery reconciles all interrupted requests. I would capture saturation of MySQL, Redis command latency, lock contention, and application goroutines to identify the first bottleneck.

## Kafka And Asynchronous Operations

### What happens if Ticket Service crashes after writing the ticket but before publishing Kafka?

The booking can exist without a `ticket.purchased` event, so the confirmation email may never be sent. Starting a goroutine to publish after persistence leaves a dual-write gap; producer acknowledgements and retries do not close that gap because the process may fail before attempting publication. I would write an outbox row in the same transaction as the durable ticket/reservation state. A relay publishes the row with retries and marks it delivered, making an undelivered event observable and recoverable.

### Explain the transactional outbox pattern and how you would operate it.

The transaction that changes business state also inserts an outbox row containing the event payload, schema version, event ID, creation time, and delivery status. A separate relay claims pending rows, publishes to Kafka, and records delivery atomically enough to tolerate duplicate publication. Consumers must still be idempotent. I would monitor pending count and age, relay error rate, publish latency, and oldest-undelivered row. The runbook distinguishes a relay problem, broker problem, and malformed payload, and avoids deleting rows until retention and replay policies permit it.

### What delivery semantics do you provide?

The consumer model is at-least-once: it commits an offset after the handler succeeds, so a crash before commit produces redelivery. It is not exactly once. The producer side currently has a dual-write loss window, so it is not an end-to-end delivery guarantee. The correct production position is at-least-once event processing plus durable idempotency, backed by an outbox to avoid silently losing business events.

### How can duplicates produce duplicate emails, and how would you stop them?

The current deduplication is a read-before-write check for an existing booking reference. Concurrent consumers can both observe no row and both send; the column is indexed but not uniquely constrained. I would use a stable UUID event ID or booking ID, add a unique database constraint, and atomically claim work with an insert/upsert before sending. I would also use an email-provider idempotency key where available. This makes redelivery safe even when the consumer commits late or restarts.

### How would you design retry topics and a DLQ?

Transient failures should have bounded attempts with delayed retry, either via scheduled retry workers or tiered retry topics. Each message carries event ID, attempt count, first-seen timestamp, failure reason, and correlation ID. After the maximum count or age, the consumer sends it to a DLQ and commits the original message so one poison message cannot block a partition. The DLQ has restricted access because payloads may contain PII, alerting on volume and age, a replay tool that preserves idempotency, and an owner/SLA for disposition.

### Which Kafka metrics would page you versus create a ticket?

I would page on sustained consumer lag that threatens the email-confirmation SLO, broker unavailability, failed outbox delivery above a threshold, or rapidly growing DLQ volume. I would create a ticket for low, stable lag, schema-deprecation usage, small transient retry increases, or capacity trends. Alerts must combine rate, duration, and customer impact; a lag number alone is not a page without knowing arrival rate, partition ownership, and the allowed end-to-end delivery delay.

### How would you evolve event schemas safely?

I would version schemas and use compatibility rules, usually backward compatibility first: new consumers tolerate absent fields and producers do not remove or change meaning of existing fields. I would add optional fields, deploy consumers that understand both versions, then deploy producers. Breaking changes use a new topic or event version with a migration window. A schema registry or contract-test gate validates compatibility in CI, while dashboards show remaining consumers of older versions.

## SRE And MLOps Operations

### Define SLIs and SLOs for this platform.

For purchase, I would measure availability of valid purchase responses, p50/p95/p99 latency, and the correctness invariant: accepted allocations must not exceed inventory. For inventory, the error budget includes any negative count, stranded reservation, or reconciliation mismatch, with a near-zero tolerance for corruption. For email, I would measure end-to-end confirmation latency from ticket confirmation to provider acceptance, outbox age, consumer lag, retry rate, and terminal failures. These become SLOs only after product agreement, for example 99.9% purchase availability and 99% of confirmations accepted by the provider within five minutes, excluding explicitly invalid addresses.

### How do you distinguish application, Redis, MySQL, Kafka, and network degradation?

I start with the request path and correlate metrics, traces, and structured logs using the correlation ID. Application issues show increased internal errors or latency without dependency degradation. Redis issues show command latency/errors, connection-pool saturation, lock acquisition anomalies, and session failures. MySQL issues show query latency, lock waits, connection exhaustion, replication or storage alarms, and failed conditional updates beyond expected sold-out behavior. Kafka issues show producer errors, outbox backlog, consumer lag, under-replicated partitions in production, or rebalance churn. Network failures normally appear as timeouts and connection errors across multiple dependencies, often localized by zone or service. I verify with dependency health, recent deploys, and synthetic probes before mitigation.

### What dashboards would you build first?

First, a service overview with RED metrics: request rate, error rate by endpoint/status, and latency percentiles. Second, a purchase dashboard showing successful reservations, sold-out responses, lock conflicts, conditional-update failures, and reconciliation mismatches. Third, an asynchronous-delivery dashboard for outbox age, Kafka lag, processing rate, retry rate, DLQ count, and confirmation delay. Fourth, dependency dashboards for Redis and MySQL saturation and error rates. I would link each panel to a runbook and avoid dashboards that lack a clear operational decision.

### How would you trace one booking end to end?

At the API boundary I generate or accept a correlation ID, propagate it in HTTP context, structured logs, database audit/outbox records, Kafka headers, and the email-delivery status record. I would add distributed tracing with spans for authentication, lock acquisition, conditional inventory update, ticket persistence, outbox relay, Kafka consumption, and provider send. Sensitive values such as session tokens and email addresses must not be trace attributes. A support operator should be able to start with a booking reference and retrieve state transitions without exposing PII.

### What alerts would you set for lock conflicts, lag, retries, and latency?

I would set thresholds from a baseline and tie them to the SLO, not invent universal numbers. Examples: page on purchase error-budget burn or sustained p99 latency; alert urgently when inventory reconciliation detects mismatch; page when outbox age or Kafka lag means confirmations will breach their latency SLO; and ticket on a sustained rise in lock conflicts because it signals a hot event or capacity constraint. Email retry and DLQ alerts consider both rate and oldest-message age. Each alert needs a severity, owner, runbook, and suppression policy during known maintenance.

### Design an incident runbook for growing Kafka lag.

First confirm consumer lag by group and partition, arrival rate, message age, and whether customer-facing confirmation SLOs are threatened. Check consumer health, restart/rebalance churn, handler latency, downstream MySQL/email provider failures, broker health, and recent producer/schema/deployment changes. Mitigate the bottleneck: restore failed consumers, scale consumers up to partition count, pause nonessential consumers, rate-limit producers only if necessary, or isolate poison messages into a DLQ. Do not blindly reset offsets; preserve evidence and replay safely through idempotent processing. Afterwards, document time to detect, customer impact, root cause, capacity change, and test/runbook improvements.

### How would you run a game day for Redis, Kafka, and MySQL failures?

I would start in a production-like non-production environment with hypotheses, success criteria, observers, rollback controls, and no real PII. For Redis, inject latency and unavailability and verify protected endpoints follow the documented fail-closed policy while alerts fire. For Kafka, stop consumers or deny producer access and verify the outbox retains events, backlog age alerts, and recovery/replay. For MySQL, introduce connection exhaustion, query latency, and a controlled failover; verify timeouts, transaction behavior, purchase rejection policy, and reconciliation. Each exercise ends with measured gaps, owned follow-ups, and an updated runbook.

### Where should the platform fail open or fail closed?

Ticket allocation must fail closed: uncertain state is better than overselling. Session validation should normally fail closed for protected endpoints because accepting an unverifiable session is a security risk. The current rate limiter fails open on Redis errors, which may be acceptable only when protected by upstream WAF/gateway controls; otherwise I would prefer a conservative local fallback or fail closed for sensitive login/registration routes. This is a product and security decision that must be explicit, tested, and observable rather than an accidental result of error handling.

### How would you capacity-plan a high-demand event?

I would establish a workload model: expected request burst, authenticated-session ratio, conversion rate, inventory size, response SLO, retry behavior, and partition/key distribution. I would benchmark the full critical path, then measure application CPU/memory/goroutines, Redis command latency, MySQL query latency/locks/connections, and Kafka/outbox throughput. The current per-event lease intentionally serializes a hot event, so it is likely the first throughput ceiling. The long-term solution is not simply more lock shards; it is a reservation or queue-based allocation architecture that preserves a single inventory authority while controlling admission and user experience.

### How would you make it multi-AZ and then multi-region?

I would first use managed multi-AZ MySQL, Redis, and Kafka or equivalents in one region, with backups, tested restores, immutable deployments, and clearly defined RTO/RPO. Multi-region comes only from a quantified latency or availability requirement. For inventory, one region or partition must remain authoritative per event; active-active writes to the same finite inventory create conflict complexity. Other regions can route purchase requests to the authority and use replicated read models. Failover requires explicit ownership transfer, fencing, recovery procedures, and tests for split-brain scenarios.

## Delivery, Security, And Production Readiness

### CI builds, tests, and publishes images. What is missing for safe CD?

The current pipeline builds, vets, runs tests, and publishes four images tagged with commit SHA and `latest`; it does not deploy. Safe CD needs immutable artifact promotion, environment-specific configuration and secrets, migration gates, image/dependency scanning, SBOMs, image signing/provenance, staging deployment, smoke and contract tests, progressive rollout, monitoring-based rollback criteria, and auditability. A SHA tag is more traceable than `latest`, but it is not sufficient deployment safety by itself.

### How do you promote and roll back immutable artifacts?

Build once and identify the artifact by digest, not a mutable tag. Promote that exact digest from development to staging to production after automated and required approval gates. Keep deployment manifests/configuration versioned, record who promoted what and when, and use canary or blue/green rollout with SLO-based analysis. Rollback re-deploys the prior known-good digest; schema changes must be backward-compatible or have an independently tested rollback/forward-fix strategy. Never assume code rollback can undo a destructive migration.

### What tests are missing because the suite is mock-heavy?

The fast suite does not fully validate real MySQL migrations and cross-database partial failures, Redis lease expiry/ownership behavior, Kafka producer acknowledgement and consumer commit semantics, actual schema compatibility, outbox recovery, or the complete purchase-to-email path. I would retain fast unit tests and add a smaller mandatory real-infrastructure CI lane using containers or an ephemeral environment. It would cover the highest-risk invariants, not attempt to make every test end-to-end.

### How would you test migrations, lock expiry, redelivery, and outbox recovery in CI?

Run migrations from an empty database and from a representative prior schema, assert both forward compatibility and application startup, then run rollback/forward-fix checks where supported. Use real Redis to test unique owner-token release, TTL expiry, and stale-token rejection by the inventory authority. Use real Kafka to crash/restart a consumer before offset commit and verify one business effect via a unique idempotency key. For outbox recovery, persist business state and an outbox row, stop the relay before publishing, restart it, and assert eventual publication exactly once from the consumer's business perspective.

### How should secrets be stored, rotated, audited, and injected?

Local Compose credentials are development-only and not a production design. In production I would store secrets in a managed secret system or KMS-backed platform integration, authenticate workloads by identity rather than long-lived shared credentials, inject secrets at runtime with least privilege, and never bake them into images or logs. Rotation must be designed per credential type: support overlapping key versions, test rotation regularly, audit access, alert on failed retrieval/expiry, and revoke compromised credentials quickly. Database users, Kafka ACLs, and Redis access should be separate by service.

### Why is raw SHA-256 for email lookup risky, and what would you use?

Email addresses have a relatively predictable input space. A deterministic unsalted SHA-256 lookup value can be dictionary-enumerated if the database is exposed. I would use a keyed HMAC with a separately managed lookup key, while keeping AES-GCM ciphertext for the actual email value. This retains deterministic lookup behavior without allowing an attacker to validate guesses without the HMAC key. Key rotation needs a versioned migration strategy and a temporary dual-lookup period if necessary.

### Describe the encryption-key rotation plan and key-loss recovery.

Each ciphertext should carry a key ID/version. New writes use the active key; a background job decrypts old rows using their recorded key and re-encrypts with the new version, with progress and error metrics. Keys should be protected by KMS/Vault envelope encryption and access should be audited. Recovery from key loss is mostly prevention: maintain governed backup and recovery controls for keys, test restoration, and document the impact. If a key is irretrievably lost, affected encrypted data may be unrecoverable, so the incident plan must cover user impact, retention requirements, and whether data must be deleted or restored from protected backup.

### What PII might enter logs, and how do you stop it?

The current stdout email provider can print recipients and message content, so it is not suitable for production. Request logging can also accidentally capture authorization headers, session tokens, bodies, query parameters, stack traces, and decrypted user fields. I would define an allowlist of safe log fields, redact known sensitive keys at the logging boundary, prevent request-body logging by default, tokenize correlation identifiers, enforce retention/access controls, and add automated tests or static checks for common PII/secrets in logs.

## Closing Questions

### What is the most important improvement you would make next?

I would prioritize correctness and recovery on the booking path: add an idempotency key, move to a transactional reservation/inventory authority, and write an outbox record in the same transaction. That addresses the highest-impact gaps: cross-database partial success, duplicate retries, and lost post-booking events. In parallel, I would add real-infrastructure tests and the minimal observability needed to detect regressions.

### What would you do differently if you rebuilt this project?

I would still begin with the inventory invariant and a fast local developer workflow, but I would model reservations explicitly from the start, make the outbox part of the initial state transition, and define operational SLIs before selecting infrastructure. I would use Redis only as a contention optimization, not as a basis for a strong correctness claim. I would also separate local-development convenience from production posture earlier: least-privilege identities, immutable deployment artifacts, infrastructure-backed critical-path tests, and runbooks for the dependencies that determine availability.
