---
title: "Redis"
category: "entity"
tags: [infrastructure, cache, sessions]
source_count: 10
updated: 2026-11-16
---

# Redis

## Overview

Redis stores user sessions and supports auth middleware. It is not part of the ticket purchase concurrency path; MySQL reservation-unit rows are the authority for available inventory.

## Responsibilities

- Store opaque session tokens with a 24-hour TTL
- Refresh session TTL on authenticated activity
- Revoke sessions immediately on logout
- Support rate limiting in user-facing middleware

## Interfaces

Auth middleware reads `Authorization: Bearer <token>`, looks up the session key in Redis, and injects the user context into the request.

## Data Model

Session keys map opaque tokens to user identity and expire automatically. See [[session-management]].

## Key Decisions

- Redis is retained for session lifecycle and middleware needs.
- Ticket inventory correctness does not depend on Redis; purchase claims and commits happen in MySQL. See [[reservation-unit-pool]].
- The four additional Redlock Redis containers were removed from Docker Compose because purchase coordination no longer uses them.

## Cross-references

- [[session-management]] — opaque session tokens and rolling TTL
- [[reservation-unit-pool]] — MySQL inventory authority
- [[user-service]] — session producer
- [[ticket-service]] — session validation only
- [[sources/config-files]] — single Redis container configuration
