# Interview guide

## The 20-second explanation

PulsePoll is a React and Go live polling app where MongoDB is durable truth and Redis is the realtime accelerator. A transaction stores each unique ballot, increments a monotonic version, and writes an outbox event. A retrying relay applies the full snapshot to Redis with idempotent Lua and publishes it to WebSocket viewers. Reconnect compares versions and repairs from REST.

## Questions worth expecting

### Why not increment Redis first?

Because a later database failure could lose a vote the API already accepted. The database transaction defines one explainable commitment point. Redis failure delays live delivery but does not change truth.

### Why both a unique index and an idempotent event?

They protect different seams. The unique `(poll_id, voter_id)` index prevents two durable ballots. The Redis applied-event key prevents a retrying relay from broadcasting/applying the same outbox event twice.

### Why publish a full snapshot instead of only a delta?

A full snapshot costs more bytes but makes gaps repairable and event order less fragile. The client still checks increasing versions. This is a good trade for polls with at most ten options.

### Is one vote per browser secure?

It is abuse friction, not identity. Clearing cookies or switching browsers creates a new voter identity. Stronger guarantees require login, invite codes, or verified identity and would change participation friction and privacy.

### What happens when Redis fails?

The vote transaction can still commit and return accepted. The outbox remains pending. The relay backs off and later replaces Redis with the durable full snapshot. WebSocket viewers reconnect and reconcile.

### What happens when MongoDB fails?

The vote is not accepted and Redis is not incremented. Returning an error is safer than showing a fast but false success.

### Can the API scale horizontally?

Yes. MongoDB owns uniqueness/transactions, relay claims are atomic, Redis Pub/Sub reaches subscribers on every replica, and no correctness state exists only in one Go process. WebSocket connections remain local to a replica but all receive the shared event.

### Why opaque sessions instead of JWT?

Revocation is immediate, stored browser credentials are hashed, and authorization does not depend on a long-lived self-contained claim. It costs one indexed session lookup per protected request, which is acceptable here.

### Why WebSocket rather than SSE?

SSE would satisfy one-way data flow with less protocol machinery. WebSocket was chosen to demonstrate explicit connection health, ping/pong, bounded delivery, and the requested architecture. The REST snapshot remains the recovery path either way.

### What would you change for real production?

Use paid warm compute and backed-up Mongo/Redis with SLAs, add metrics/alerts and distributed tracing, partition relay ownership for larger throughput, define data-retention jobs, run abuse testing, and choose identity controls with the customer. None requires moving the vote acceptance boundary to Redis.

## Claims to avoid

- Do not call a free-tier deployment highly available.
- Do not say a cookie proves one person.
- Do not claim a load-test latency unless the raw run and environment are recorded.
- Do not say Pub/Sub guarantees delivery; the versioned snapshot is what makes loss safe.
- Do not hide the use of AI. Explain which code and tests you personally inspected and can defend.
