# Submission readiness and evidence matrix

This page separates verified evidence from work that requires an external account. It is deliberately strict: a green local build is not called a public deployment.

## Verified in the repository

| Requirement | Status | Authoritative evidence |
|---|---|---|
| Account and owner boundaries | Verified | Security integration tests cover unauthenticated access, CSRF, ownership, session rotation, and generic credentials. |
| Poll lifecycle | Verified | Desktop and mobile Playwright flows create, share, vote, update live, close, and reject a late vote; dashboard exposes reopen, export, and archive. |
| Durable voting | Verified | The concurrency test runs 100 voters plus duplicate retries and reconciles durable MongoDB totals, Redis, outbox events, and WebSocket versions. |
| Reconnect recovery | Verified | Integration coverage disconnects a viewer, misses a vote, reconnects, and verifies the replacement snapshot. |
| Redis outage recovery | Verified | `scripts/redis-failure-drill.sh` accepts a vote with Redis stopped, restarts Redis, and waits for exact version/count repair. |
| Security and supply chain | Verified | `go test -race`, `go vet`, `govulncheck`, `npm audit`, bounded JSON tests, exact-origin tests, and digest-pinned containers run in CI. |
| Desktop and mobile UX | Verified | Playwright runs the complete flow in Desktop Chrome and Pixel 7 profiles; checked-in screenshots show landing, poll, mobile, and dashboard states. |
| Load and reconnect targets | Verified locally | Raw k6 evidence records 401/401 accepted votes, 100/100 WebSocket snapshots, zero failed checks, 10.09 ms warm p95, and exact MongoDB/Redis reconciliation. |
| Reproducible demo | Verified | `scripts/demo.sh reset` and `seed` maintain only `[DEMO]` data and refuse the default credential on remote hosts. |
| 3–5 minute video artifact | Verified locally | `scripts/build_demo_video.py` generates a 1920×1080 H.264/AAC walkthrough from checked-in evidence; the verified export is 298.7 seconds. |

## External gates before emailing the submission

| Gate | Current state | Required action |
|---|---|---|
| Public repository default branch | PR ready | Merge the green `codex/production-rebuild` pull request into the public repository’s `main` branch. |
| Public HTTPS/WS URL | Not provisioned | Provision free Atlas, TLS Redis, and Render resources; add the four secret environment variables from `docs/DEPLOYMENT.md`; incur no paid plan. |
| Live verification | Waiting on URL | Run `scripts/verify-public-deployment.sh https://…`, then run Playwright with `PLAYWRIGHT_BASE_URL=https://…`. Test incognito, mobile, cold start, reconnect, close, CSV, and totals. |
| Final release | Release candidate first | Promote the verified commit to `v1.0.0` only after the public URL passes. Do not label an undeployed candidate as final. |
| Candidate-owned narration | Fallback available | Watch the generated video and preferably replace the generated voice with the candidate’s own explanation before submission. |

## Honest AI disclosure

Suggested wording:

> I used an AI coding assistant for implementation acceleration, test generation, review, and documentation. I personally reviewed the architecture and evidence, ran the tests, and can explain the MongoDB transaction boundary, outbox relay, Redis Lua idempotency, WebSocket recovery protocol, and security trade-offs.

## Final three-link check

Immediately before emailing `devhiring@hclguvi.com`, open all three links in a signed-out browser:

1. Public GitHub repository on the final tagged commit.
2. Public HTTPS app URL with a successful fresh-account, two-viewer rehearsal.
3. Publicly accessible 3–5 minute video.

Record the commit SHA, release tag, deployment URL, video URL, UTC verification time, and verifier name in the submission email. Never publish credentials, cookies, or provider connection strings.
