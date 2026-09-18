# Verification evidence

## Local k6 vote burst — 18 September 2026

This is a measured local result, not a free-tier or public-cloud claim.

| Property | Recorded value |
|---|---:|
| Environment | macOS 26.5.1, Apple arm64, Docker Desktop 29.2.1 |
| State | Warm local Docker Compose stack |
| k6 image | `grafana/k6:2.2.0` |
| Vote arrival rate | 20 new browser identities/second for 20 seconds |
| Reconnect arrival rate | 5 WebSocket viewers/second for 20 seconds |
| Completed votes | 401/401 accepted |
| Reconnect sessions | 100/100 received a snapshot |
| HTTP failure rate | 0.00% |
| WebSocket failure rate | 0.00% |
| Vote latency p95 | 10.09 ms |
| Vote latency max | 40.82 ms |
| Count reconciliation | MongoDB 401 = Redis 401 after outbox drain |

The raw machine-readable output is [k6-local-summary.json](k6-local-summary.json). Reproduce it with:

```bash
docker compose up -d --build
RATE=20 VIEWER_RATE=5 DURATION=20s scripts/run-local-loadtest.sh
```

Performance varies by hardware and deployment region. The checked-in thresholds are evaluation goals (`<1%` request failures and warm p95 `<400 ms`), not a production SLA.
