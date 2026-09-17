# Verification evidence

## Local k6 vote burst — 18 September 2026

This is a measured local result, not a free-tier or public-cloud claim.

| Property | Recorded value |
|---|---:|
| Environment | macOS 26.5.1, Apple arm64, Docker Desktop 29.2.1 |
| State | Warm local Docker Compose stack |
| k6 image | `grafana/k6:2.2.0` |
| Arrival rate | 20 new browser identities/second for 20 seconds |
| Completed votes | 400/400 accepted |
| HTTP failure rate | 0.00% |
| Vote latency p95 | 11.29 ms |
| Vote latency max | 18.94 ms |
| Count reconciliation | MongoDB 400 = Redis 400 after outbox drain |

The raw machine-readable output is [k6-local-summary.json](k6-local-summary.json). Reproduce it with:

```bash
docker compose up -d --build
RATE=20 DURATION=20s scripts/run-local-loadtest.sh
```

Performance varies by hardware and deployment region. The checked-in thresholds are evaluation goals (`<1%` request failures and warm p95 `<400 ms`), not a production SLA.
