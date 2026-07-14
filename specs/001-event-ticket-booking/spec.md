# Feature Specification: Event Ticket Booking Platform

**Feature Branch**: `001-event-ticket-booking`

**Created**: 2025-07-14

**Status**: Draft

**Input**: User description: "Build an application where users can register an account, browse available event tickets, and purchase them concurrently without overselling. User journeys: account creation with secure PII storage, concurrent ticket purchasing with correct allocation, and asynchronous confirmation email delivery."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - User Registration with Secure Data (Priority: P1)

A visitor creates an account by providing their name, email address, and a password. Their
personal information is stored securely so it cannot be read in plaintext by anyone with
database access. The user receives immediate confirmation that their account has been created
and can then log in.

**Why this priority**: Account creation is the foundational prerequisite for ticket purchasing.
Without registration, users cannot be identified, tickets cannot be associated with a buyer, and
no downstream flows can function.

**Independent Test**: Can be fully tested by submitting a registration form and verifying that
(a) the user can log in with the same credentials, and (b) the email and other PII stored in
the backing data store are not in plaintext. Delivers value as a standalone identity system.

**Acceptance Scenarios**:

1. **Given** a visitor is on the registration page, **When** they submit a valid name, email,
   and password, **Then** an account is created, the user's PII is encrypted at rest, and the
   user sees a confirmation message and can log in.
2. **Given** a visitor is on the registration page, **When** they submit an email that is
   already registered, **Then** the system rejects the submission with a clear error message
   and does not create a duplicate account.
3. **Given** a visitor is on the registration page, **When** they submit a password that is too
   short (fewer than 8 characters), **Then** the system rejects the submission with a
   validation error and explains the password requirements.
4. **Given** an authenticated user, **When** they log out and attempt to access a protected
    page, **Then** the system redirects them to the login page.
5. **Given** a client has made 5 failed login attempts within one minute, **When** they attempt
   a 6th login, **Then** the system returns a rate-limit error with a retry-after indication.

---

### User Story 2 - Browse Available Events (Priority: P2)

Any visitor (authenticated or not) browses a list of upcoming events. Each event displays its
name, date, venue, description, and the number of remaining tickets. The user can click into an
event for more detail and to see the purchase option (which requires login if not already
authenticated).

**Why this priority**: Browsing events is the discovery mechanism that leads to ticket
purchases. It is independently valuable — a visitor can browse without buying, and it serves as
the entry point for the purchasing flow. It has no dependency on the purchase logic.

**Independent Test**: Can be fully tested by creating several events with varying ticket
availability in the system, then verifying that the event list displays all events with
accurate remaining-ticket counts — accessible without authentication. Delivers value as a
standalone public event catalog.

**Acceptance Scenarios**:

1. **Given** events exist in the system with defined ticket quantities, **When** a user views
   the event list, **Then** all upcoming events are displayed with their name, date, venue, and
   remaining ticket count.
2. **Given** an event has zero remaining tickets, **When** a user views the event list, **Then**
   the event is still listed but marked as "Sold Out" with no purchase option available.
3. **Given** a user is viewing the event list, **When** they click on a specific event, **Then**
   they see the event's full details including description and an option to purchase if tickets
   remain.

---

### User Story 3 - Concurrent Ticket Purchase (Priority: P3)

An authenticated user selects an event, specifies the number of tickets they want (up to the
available quantity), and confirms the purchase. When multiple users attempt to purchase the
last remaining tickets simultaneously, the system guarantees that no ticket is sold twice — the
fastest valid purchase requests succeed and the rest are correctly declined. After a successful
purchase, a confirmation email is sent to the user asynchronously so the checkout process
completes without delay.

**Why this priority**: This is the core value proposition — fair ticket allocation under
concurrent load. It is listed last because it depends on P1 (registration) and is most valuable
when P2 (browsing) is in place. However, it is the primary differentiator for the platform.

**Independent Test**: Can be fully tested by creating an event with exactly 1 ticket remaining,
simulating 10+ users attempting to purchase that ticket simultaneously, and verifying that
(a) exactly 1 purchase succeeds, (b) the other 9+ are declined with a clear "sold out" or
"no longer available" message, (c) the successful buyer's ticket count is decremented
correctly, and (d) the successful buyer receives a confirmation email within a reasonable time
window. Delivers value as the core transactional engine.

**Acceptance Scenarios**:

1. **Given** an event has N remaining tickets and exactly N users each request 1 ticket
   sequentially, **When** all N purchase requests complete, **Then** all N users receive
   confirmation and the event shows 0 remaining tickets.
2. **Given** an event has exactly 1 remaining ticket and 10 users attempt to purchase it
   concurrently, **When** all requests are processed, **Then** exactly 1 user succeeds, the
   event shows 0 remaining tickets, and the other 9 users receive a clear "ticket unavailable"
   response.
3. **Given** an event has 5 remaining tickets and a user attempts to purchase 6, **When** the
   request is processed, **Then** the purchase is declined with a message indicating
   insufficient availability, and 5 tickets remain.
4. **Given** a successful ticket purchase, **When** the purchase is confirmed, **Then** a
   confirmation email is sent to the user without delaying the purchase response, and the email
   contains the event name, date, venue, quantity purchased, and a unique booking reference.
5. **Given** a user completes a purchase, **When** they view their account, **Then** they can
   see a list of their purchased tickets with event details and booking references.
6. **Given** a purchase is declined due to concurrent allocation, **When** the user retries
   the purchase, **Then** the system reflects the current (possibly sold-out) state and does
   not allow the purchase.

---

### Edge Cases

- What happens when a user's session expires mid-purchase?
- What happens when the email delivery service is temporarily unavailable during a successful
  purchase? The purchase must still be confirmed; email delivery should be retried.
- What happens when a user with an existing account attempts to register with the same email?
- What happens when a user attempts to purchase tickets for a past event?
- What happens when a distributed lock cannot be acquired within the timeout window?
- What happens when a user's password is changed — are existing sessions invalidated?
- What happens when an event's details (date, venue) are updated after tickets have been sold?
- What happens when a user purchases zero or negative tickets?
- What happens when a user exceeds the rate limit on login or registration — do they receive a clear retry-after message?
- What happens when a user requests account deletion — are their existing tickets still valid and viewable in purchase history?
- What happens when an email delivery permanently fails after all retries (dead-letter) — is there a mechanism for support staff to discover and resend?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow visitors to create an account with name, email, and password.
- **FR-002**: System MUST encrypt all PII (email, name) at rest so plaintext values are never
  exposed in the database.
- **FR-003**: System MUST reject duplicate email registrations with a clear error message.
- **FR-004**: System MUST enforce a minimum password length of 8 characters.
- **FR-005**: System MUST allow registered users to log in and log out.
- **FR-006**: System MUST display a list of upcoming events with name, date, venue, and
  remaining ticket count.
- **FR-007**: System MUST allow users to view full event details including description.
- **FR-008**: System MUST prevent purchase when an event is sold out (remaining tickets = 0)
  and display a "Sold Out" indication.
- **FR-009**: System MUST guarantee that no ticket is sold more than once, even when
  multiple users attempt to purchase the same tickets simultaneously.
- **FR-010**: System MUST decline purchase requests when the requested quantity exceeds
  available tickets.
- **FR-011**: System MUST send a confirmation email to the buyer after a successful purchase.
- **FR-012**: Confirmation email delivery MUST be asynchronous and MUST NOT delay the purchase
  response to the user.
- **FR-013**: Confirmation emails MUST include event name, date, venue, quantity purchased, and
  a unique booking reference.
- **FR-014**: System MUST allow users to view their purchase history with event details and
  booking references.
- **FR-015**: System MUST prevent purchase of tickets for events that have already occurred.
- **FR-016**: System MUST retry failed email deliveries with a reasonable backoff strategy and
  MUST NOT lose purchase confirmation even when email delivery fails.
- **FR-017**: System MUST reject invalid purchase quantities (zero or negative numbers) with a
  clear error message.
- **FR-018**: System MUST allow unauthenticated visitors to browse event listings and view
  event details. Purchase of tickets MUST require authentication (login or account creation).
- **FR-019**: System MUST rate-limit authentication endpoints per client IP address: login
  limited to 5 attempts per minute; registration limited to 3 attempts per minute.
  Rate-limited responses MUST include a clear error message indicating when the client can
  retry.
- **FR-020**: System MUST allow authenticated users to request account deletion. Upon deletion,
  PII (name, email) MUST be anonymized. Ticket purchase records MUST be preserved for audit
  purposes but disassociated from personal identity (user identifier anonymized). Data
  retention MUST comply with Singapore PDPA — PII MUST only be retained as long as necessary
  to fulfill the stated purpose of ticket purchasing and account management.

### Key Entities *(include if feature involves data)*

- **User**: Represents a registered account holder. Key attributes: unique identifier, name
  (encrypted), email (encrypted), hashed password, registration timestamp.
- **Event**: Represents a scheduled event with tickets available. Key attributes: unique
  identifier, name, description, date, venue, total ticket capacity, remaining ticket count.
- **Ticket**: Represents a purchased ticket allocated to a specific user. Key attributes: unique
  identifier (booking reference), associated event, associated user, quantity, purchase
  timestamp, status (confirmed/cancelled).
- **Confirmation Email**: Represents an asynchronous notification triggered by a purchase
  event. Key attributes: recipient, event details, booking reference, delivery status
  (pending/sent/failed), retry count.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create an account in under 1 minute.
- **SC-002**: 100% of PII stored in the data store is encrypted at rest (verifiable by
  inspection of stored records).
- **SC-003**: Event list endpoint returns results in under 500ms (p95) for up to 100 events.
- **SC-004**: Under concurrent purchase of a single remaining ticket by 100 users, exactly 1
  purchase succeeds and the other 99 are correctly declined — verified across 100 repeated
  trials with no double-sell incidents.
- **SC-005**: Purchase confirmation response is delivered to the user in under 2 seconds
  (excluding email delivery time).
- **SC-006**: Confirmation emails are delivered within 5 minutes of purchase in 99% of cases
  under normal operating conditions.
- **SC-007**: All declined concurrent purchase attempts receive a clear, actionable error
  message (not a generic system error).
- **SC-008**: Users can view their complete purchase history with accurate booking references
  immediately after each purchase.

## Clarifications

### Session 2025-07-14

- Q: Should auth endpoints (login/register) be rate limited to prevent brute force? → A: Per-IP rate limiting — login 5 attempts/min, registration 3 attempts/min, with clear error messages.
- Q: What is the account deletion and data retention policy? → A: Comply with Singapore PDPA. Users can request account deletion; PII is anonymized upon deletion; ticket records preserved without PII linkage; PII retained only as long as necessary for the stated purpose.
- Q: Should event browsing support search/filtering beyond pagination? → A: No — pagination only for v1. Filtering deferred to future iteration.
- Q: What happens when email delivery permanently fails (dead-letter)? → A: Log the dead-letter event with correlation ID. No automated alerting for v1. Dead-letter records remain queryable in the database for manual investigation.

## Assumptions

- **SC-001** (account creation in under 1 minute) is a post-launch UX metric verified through user testing, not a build-time criterion.
- Users have a stable internet connection and a modern web browser.
- Email delivery is handled by a third-party email service provider; this system is only
  responsible for submitting the send request and handling retries on failure.
- Events are pre-configured by an administrator or seed data — event creation/management is
  out of scope for this feature.
- Payment processing is out of scope. "Purchase" in this feature means reserving/claiming a
  ticket. Payment integration will be added in a future iteration.
- The system operates in a single geographic region and does not require multi-region
  concurrency coordination.
- Password-based authentication is sufficient for v1; no social login or multi-factor
  authentication is required at this stage.
- Initial event inventory and ticket quantities are assumed to be loaded via seed data or an
  administrative backdoor for the purposes of this feature.
