# PulsePoll

A live polling tool built for the GUVI Developer Internship task.
Create a poll, share a link, and watch the results update in real-time — no refresh required.

## Quick Start

### Prerequisites
- Docker + Docker Compose (Docker Desktop is enough)

### Run everything locally

```bash
git clone <repo-url> && cd PulsePoll
docker compose up --build
```

The app is available at **http://localhost**.
- `frontend` (React) is served by nginx on port 80
- `backend` (Go + Gin) runs on port 8080 *inside the compose network* (no host
  port is published – nginx proxies `/api` to it, so nothing conflicts with
  other services on your machine)
- `mongo` and `redis` run internally too — inspect them with
  `docker compose exec mongo mongosh`, `docker compose exec redis redis-cli`

If you want to poke the raw API from your host during a compose run, publish a
host port for the backend, e.g. add `ports: ["8080:8080"]` to the `backend`
service.

### Run natively (frontend hot-reload, no Docker build overhead)

The default `docker compose up` maps no host ports, so nothing collides with
other projects. For the full hot-reload native development loop, use the dev
override which publishes non-default ports:

```bash
# 1. Start the three infrastructure+backend containers with dev ports
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

# 2. In a new terminal – run the Go backend directly against those ports
cd backend
MONGO_URI=mongodb://localhost:27019 REDIS_ADDR=localhost:6390 PORT=8082 go run ./cmd/server

# 3. In a third terminal – run Vite, pointing its proxy at the backend
cd frontend
VITE_PROXY_TARGET=http://localhost:8082 npm run dev
```

Then open **http://localhost:5173** – the Vite dev server proxies `/api` to
the Go backend, so cookies and SSE work with no CORS configuration needed.
Edit React components and they hot-reload instantly.

---

## Project Structure

```
PulsePoll/
├── backend/                  # Go service
│   ├── cmd/server/main.go    # Wires everything and starts the server
│   └── internal/
│       ├── config/env.go     # Loads configuration from environment variables
│       ├── database/         # MongoDB + Redis connection and index setup
│       ├── handlers/         # HTTP handlers (auth, poll CRUD, SSE stream)
│       ├── middleware/        # JWT auth, CORS
│       ├── models/           # Data types (User, Poll, Vote)
│       ├── realtime/         # Redis pub/sub hub + vote counting
│       └── utils/            # ID generation, password hashing, cookie helpers
├── frontend/                 # React app (Vite + TypeScript)
│   └── src/
│       ├── components/       # Navbar, ResultBar
│       ├── lib/              # API client, auth context, types
│       └── pages/            # Landing, Login, Signup, Dashboard, CreatePoll, PollView
├── docker-compose.yml        # Full local stack
├── deploy/                   # Render / Fly.io deploy hints
└── README.md
```

---

## Key Decisions

### Realtime architecture: Redis Pub/Sub → SSE

Votes arrive at the Go backend via a normal POST request. The handler performs
an **atomic Redis `HINCRBY`** to bump the vote count for the selected option,
`INCR` for the total, and `SADD` to a per-poll deduplication set (preventing
a single browser from voting twice). Immediately after, a `PUBLISH` on a
Redis channel sends the full updated counts to every connected Server-Sent Events (SSE)
stream. The SSE clients receive a `data:` frame with the counts JSON and update
the UI without a page refresh.

This means:
- **Redis is the source of truth for live counts.** Reads for the live UI come
  straight from Redis (`HGETALL`) for speed; the same key is also updated for
  every vote.
- **Redis Pub/Sub is doing real work.** It acts as the fan-out bus that
  distributes votes to all connected viewers — including across multiple backend
  instances if the app scales horizontally.
- **MongoDB is the durable ledger.** Poll definitions and every individual vote
  record live in Mongo. Mongo counts are maintained in sync via `$inc` with
  `arrayFilters` so they stay correct if Redis were ever lost.

### Auth: JWT httpOnly cookie

A short-lived (7-day) JWT is set as an httpOnly cookie on login or signup.
The same cookie is read by the auth middleware on protected endpoints and the
Vote handler reads an anonymous voter-ID cookie (`ppv`) for duplicate-vote
prevention. No tokens are exposed to JavaScript.

### Slug-based poll URLs

Polls are accessed via short 8-character alphanumeric slugs (`/poll/a7b2xk9m`)
rather than opaque MongoDB ObjectIDs. Slugs are retried on collision.

---

## Deploy Guide

The repo ships with a single-service production image
(`deploy/combined.Dockerfile`) that runs **nginx + the Go backend together**
via supervisord. That means you can deploy to Render, Fly.io, or Railway as
**one** service against cloud Mongo + Redis. All you need from cloud
providers is a MongoDB (Atlas free tier) and a Redis (Redis Cloud free tier).

### Option A: Render (recommended)

1. Create a **MongoDB Atlas** cluster and a **Redis Cloud** database (free
   tiers are fine).
2. Push the repo to GitHub.
3. On Render, create a **Web Service** → **Deploy from Dockerfile**, pointing
   at `deploy/combined.Dockerfile`. A blueprint (`deploy/render.yaml`) is
   included for one-click setup.
4. Set environment variables:
   ```
   ENV=production
   MONGO_URI=mongodb+srv://<user>:<pass>@<cluster>/?retryWrites=true&w=majority
   MONGO_DB=pulsepoll
   REDIS_ADDR=<redis-cloud-host>:<port>
   REDIS_PASS=<redis-password>
   JWT_SECRET=<64-char-random-string>
   FRONTEND_ORIGINS=https://your-app.onrender.com
   ```
5. Expose port 80. Done.

### Option B: Fly.io

1. `fly launch` in the repo root, select a Dockerfile deploy.
2. Set secrets: `fly secrets set MONGO_URI=... REDIS_ADDR=... REDIS_PASS=... JWT_SECRET=... FRONTEND_ORIGINS=...`
3. `fly deploy`.

### Option C: Single VPS (any cloud box)

```bash
docker build -f deploy/combined.Dockerfile -t pulsepoll .
docker run -d -p 80:80 \
  -e ENV=production \
  -e MONGO_URI="..." -e MONGO_DB=pulsepoll \
  -e REDIS_ADDR="..." -e REDIS_PASS="..." \
  -e JWT_SECRET="..." \
  -e FRONTEND_ORIGINS="https://your-app.example.com" \
  pulsepoll
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the Go backend |
| `ENV` | `development` | `production` enables secure cookies |
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `MONGO_DB` | `pulsepoll` | MongoDB database name |
| `REDIS_ADDR` | `localhost:6379` | Redis host:port |
| `REDIS_PASS` | (empty) | Redis password |
| `JWT_SECRET` | `dev-secret-change-me` | Secret used to sign JWTs |
| `FRONTEND_ORIGINS` | `http://localhost:5173,http://localhost` | Comma-separated allowed CORS origins |