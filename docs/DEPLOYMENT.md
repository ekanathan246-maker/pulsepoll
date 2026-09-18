# Zero-cost deployment runbook

This runbook uses a single Render web service, MongoDB Atlas Free, and a TLS Redis provider such as Upstash Free. Free plans and quotas can change; verify the provider dashboards before submission and do not enable a paid plan.

## 1. Provision durable data

1. Create an Atlas Free cluster in the closest available region to Render.
2. Create a least-privilege database user for one database.
3. Allow network access only as narrowly as the selected host permits.
4. Copy the `mongodb+srv://` URI into Render as `MONGO_URI`.
5. Keep `retryWrites=true&w=majority`; transactions require a replica set, which Atlas provides.

## 2. Provision live data

1. Create one free Redis database near the API region.
2. Copy its TLS URL (`rediss://...`) into Render as `REDIS_URL`.
3. Do not expose that URL to Vite or browser variables.
4. Review command and bandwidth usage after the demo. PulsePoll bounds ephemeral keys to 30 days and applied-event keys to 24 hours.

## 3. Deploy the application

1. Push the repository to GitHub.
2. In Render choose **New → Blueprint** and select this repository.
3. Confirm `render.yaml` selects `deploy/combined.Dockerfile` and the free plan.
4. Add `MONGO_URI`, `REDIS_URL`, `FRONTEND_ORIGINS`, and `PUBLIC_URL` in the Render dashboard.
5. Set both origin variables to the exact final `https://<service>.onrender.com` URL.
6. Deploy and wait for `/api/readyz` to return `{"status":"ready"}`.

The image builds React and Go in separate stages, then runs nginx and the Go application process as UID 101 under Supervisor. nginx listens on Render's standard port 10000, serves static files, and forwards REST/WebSocket traffic to the local API on port 18080.

## 4. Release verification

- Open the public URL on desktop and mobile/incognito.
- Create a fresh account and poll; do not rely on seeded browser cookies.
- Open the share URL in another browser context and cast a vote.
- Confirm both windows show the new result without refresh.
- Turn Wi-Fi off/on or restart the page; confirm the badge returns to **Live · synced** and counts agree.
- Close the poll as owner and confirm the guest cannot vote.
- Download CSV and compare its sum with the UI total.
- Check Render JSON logs for request IDs and `/api/readyz` for dependencies.
- Recheck repository, live URL, and video permissions immediately before submission.

Render free services can sleep and produce a cold start. Warm the service manually before the interview; do not run fake keep-alive traffic.

## Redis failure drill

1. In local Compose, stop Redis: `docker compose stop redis`.
2. Cast a vote. MongoDB should accept it and create an unprocessed outbox event; the live view may reconnect.
3. Start Redis: `docker compose start redis`.
4. The relay should apply the complete snapshot and the viewer should reconcile.
5. Confirm Mongo and Redis totals match before publishing any claim.

## Rollback

1. Roll Render back to the previous known-good image/commit.
2. Do not roll database documents backward destructively.
3. The current schema is additive. Older clients ignore additive fields; if a future migration becomes incompatible, ship a forward repair migration.
4. Verify `/api/readyz`, login, one new poll, one vote, and WebSocket recovery after rollback.

## Upgrade triggers

Move from free tiers when the product needs continuous warm capacity, automated backups, private networking, an SLA, sustained load beyond quotas, or multiple regions. The durable-outbox design remains valid; the deployment topology changes, not the vote contract.
