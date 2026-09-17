# PulsePoll engineering demo narration

## 1. The product moment

PulsePoll turns one question into a shared live moment. A host signs in, creates a poll, and shares one URL or QR code. Guests do not need an account. They vote from a phone or an incognito window, and every connected viewer sees the result move without refreshing. The scope is intentionally narrow because the project is designed to make correctness, recovery, and product judgment easy to inspect.

## 2. The complete user flow

The host can create two to ten unique options, choose whether results appear before voting, schedule a closing time, copy the public link, or show a QR code. The dashboard lists owned polls and supports close, reopen, CSV export, and soft archive. Results use text labels as well as color, the interface works from a keyboard, and the same flow is tested on desktop Chromium and a mobile viewport.

## 3. Durable first, live immediately after

The most important decision is the acceptance boundary. MongoDB, not Redis, decides whether a vote exists. In one transaction, the API inserts a ballot protected by a unique poll-and-voter index, increments the durable option total and poll version, and writes an outbox event. Only after that transaction commits does the API return accepted. This means a Redis outage may delay animation, but it cannot erase a vote the user was told succeeded.

An in-process relay claims pending outbox events. A Redis Lua script applies each event ID once, replaces the hot count snapshot and version, and publishes the versioned event. Redis therefore has real responsibilities: live counts, cross-instance fan-out, idempotent event application, and rate limiting, while MongoDB remains the durable source of truth.

## 4. Reconnect and partial-failure recovery

Redis Pub Sub is fast but does not retain history. The browser never treats it as a ledger. A WebSocket connection receives a snapshot first and then increasing versions. If a version is skipped, the network drops, or the server restarts, the client enters a reconnecting state with bounded backoff, fetches the durable REST snapshot, and replaces local state. Ping and pong detect dead peers, slow connections are bounded, and a global connection cap protects the process.

The Redis failure drill proves the recovery path. It stops Redis, casts a vote, observes an honest accepted response from MongoDB, restarts Redis, and waits until the outbox repairs the cache to the same version and count.

## 5. Security boundaries

Passwords use Argon2id. Session and CSRF credentials are opaque random values, and only their hashes are stored. Login rotates an existing session. State-changing owner routes require both authentication and CSRF, ownership checks happen in database filters, CORS uses an exact allowlist, JSON rejects unknown and oversized input, and rate limits return a retry interval. A browser cookie makes repeat voting harder, but it is honestly described as abuse friction rather than proof of one human.

## 6. Evidence, not performance theatre

The integration suite launches one hundred concurrent voters, retries every ballot, and proves exactly one hundred durable votes, unique live events, and matching MongoDB and Redis totals. It then disconnects a viewer, casts a missed vote, reconnects, and verifies the recovered snapshot. Negative tests cover authentication, CSRF, ownership, invalid input, duplicate and closed-poll voting, origin checks, and session rotation.

The checked-in local k6 result accepted four hundred and one of four hundred and one votes at twenty requests per second, while one hundred reconnecting viewers all received a WebSocket snapshot. Warm vote latency was ten point zero nine milliseconds at the ninety-fifth percentile. Those numbers are labeled with the machine, runtime, warm state, and raw result; they are evidence from one local run, not a free-tier service-level promise.

## 7. Release and operations

Every push runs Go race tests, static analysis, vulnerability scanning, frontend unit tests, an npm audit, OpenAPI validation, Mongo and Redis integration tests, the Redis outage drill, desktop and mobile Playwright flows, and a production image build. The release container uses exact digest-pinned toolchains, current stable nginx, and a non-root runtime. Liveness avoids dependencies; readiness checks them and turns false during graceful shutdown. JSON metrics expose request rate, errors, latency, WebSocket clients, relay failures, and outbox lag without requiring a paid monitoring service.

## 8. Honest close

PulsePoll is production-shaped, not production-claimed. A free deployment can cold-start, free databases have quotas, and browser identity cannot prevent determined abuse. The architecture has clear upgrade points: multiple API replicas already share Redis fan-out, accepted votes remain durable in MongoDB, and stronger identity or managed infrastructure can be added without rewriting the vote path. The project’s strongest feature is that every important promise has a test, a failure behavior, and evidence that can be explained in an interview.
