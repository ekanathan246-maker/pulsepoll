# 3–5 minute demo script

## 0:00–0:35 — The product moment

Open the landing page. Say: “PulsePoll turns one question into a live room signal. A host shares one link; guests need no account.” Sign in and create a two-option poll.

## 0:35–1:20 — Two-viewer proof

Copy the link or scan the QR into an incognito/mobile window. Keep host and guest visible side by side. Vote as the guest. Point out:

- results were hidden before the vote;
- the guest received a durable `202` result;
- the host updated without refreshing;
- both screens show **Live · synced**.

## 1:20–2:10 — Backend judgment

Show the architecture diagram. Say: “Redis does real work, but it does not decide whether a vote exists. One MongoDB transaction inserts the unique ballot, increments totals and version, and creates an outbox event. Only then is the vote accepted. The relay applies the full snapshot to Redis idempotently and publishes it.”

Explain why the complete snapshot matters: version gaps repair themselves instead of replaying uncertain deltas.

## 2:10–2:50 — Failure behavior

Reload the guest or briefly go offline. Show **Reconnecting**, then **Live · synced**. Say: “Pub/Sub is at-most-once, so reconnect never trusts it as history. The client fetches durable state and replaces its local copy.”

Show the duplicate-vote response or simply try the same vote again. Explain the unique compound index and the honest browser-identity limitation.

## 2:50–3:35 — Product completion

From the dashboard show QR sharing, close/reopen, CSV export, scheduled close, responsive layout, and archive. Close the poll and verify the guest controls disable.

## 3:35–4:20 — Evidence

Open CI or the terminal summary:

- Go unit/race tests;
- 100 concurrent accepted voters plus 100 rejected duplicates, exact total 100;
- Vitest UI checks;
- Playwright two-viewer flow on desktop and mobile;
- production Docker build and readiness probe.

Do not quote performance numbers unless the checked-in k6 test was run in the same environment and the raw result is available.

## 4:20–4:45 — Honest close

Say: “The free deployment is production-shaped, not an SLA. Render can cold-start, and a browser token cannot prove one human. The architecture has explicit upgrade points without rewriting the acceptance path.”

End on the live poll, not on a slide.
