#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost}"
K6_BASE_URL="${K6_BASE_URL:-http://frontend}"
RATE="${RATE:-20}"
DURATION="${DURATION:-20s}"
VIEWER_RATE="${VIEWER_RATE:-5}"
COOKIE_FILE="$(mktemp -t pulsepoll-load-cookies.XXXXXX)"
BODY_FILE="$(mktemp -t pulsepoll-load-body.XXXXXX)"
trap 'rm -f "$COOKIE_FILE" "$BODY_FILE"' EXIT

mkdir -p docs/evidence
email="load-$(date +%s)-$RANDOM@example.test"
signup="$(jq -nc --arg email "$email" '{name:"PulsePoll Load Test",email:$email,password:"LoadTest!2026"}')"
status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -c "$COOKIE_FILE" -H 'Content-Type: application/json' --data "$signup" "$BASE_URL/api/auth/signup")"
[[ "$status" == "200" ]] || { echo "Signup failed ($status): $(cat "$BODY_FILE")" >&2; exit 1; }
csrf="$(awk '$6 == "pp_csrf" { value=$7 } END { print value }' "$COOKIE_FILE")"

poll_payload='{"title":"Local k6 durability run","options":["Accepted","Rejected"],"showResultsBeforeVote":true}'
status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -b "$COOKIE_FILE" -H 'Content-Type: application/json' -H "X-CSRF-Token: $csrf" --data "$poll_payload" "$BASE_URL/api/polls")"
[[ "$status" == "201" ]] || { echo "Poll creation failed ($status): $(cat "$BODY_FILE")" >&2; exit 1; }
slug="$(jq -r '.slug' "$BODY_FILE")"
option_id="$(jq -r '.options[0].id' "$BODY_FILE")"
frontend_container="$(docker compose ps -q frontend)"
[[ -n "$frontend_container" ]] || { echo "Compose frontend is not running" >&2; exit 1; }
compose_network="$(docker inspect "$frontend_container" --format '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}')"
[[ -n "$compose_network" ]] || { echo "Could not resolve the Compose network" >&2; exit 1; }

docker run --rm --network "$compose_network" \
  -e "BASE_URL=$K6_BASE_URL" -e "POLL_SLUG=$slug" -e "OPTION_ID=$option_id" \
  -e "RATE=$RATE" -e "DURATION=$DURATION" -e "VIEWER_RATE=$VIEWER_RATE" \
  -v "$PWD:/work" -w /work grafana/k6:2.2.0@sha256:9bd01d6941fca969cb61bb57d2da5ee9b385fe2aa8881df3798c196564d6ace6 run \
  --summary-export=/work/docs/evidence/k6-local-summary.json loadtest/vote.js

snapshot="$(curl -fsS "$BASE_URL/api/polls/$slug")"
expected="$(jq -r '.metrics.votes_accepted.count' docs/evidence/k6-local-summary.json)"
actual="$(jq -r '.totalVotes' <<<"$snapshot")"
[[ "$actual" == "$expected" ]] || { echo "Count drift: accepted k6 votes=$expected durable votes=$actual" >&2; exit 1; }
expected_version=$((actual + 1))
for _ in $(seq 1 60); do
  redis_count="$(docker compose exec -T redis redis-cli HGET "pp:poll:$slug:counts" "$option_id" 2>/dev/null || true)"
  redis_version="$(docker compose exec -T redis redis-cli GET "pp:poll:$slug:version" 2>/dev/null || true)"
  if [[ "$redis_count" == "$actual" && "$redis_version" == "$expected_version" ]]; then
    echo "PASS: $actual votes with no Mongo/Redis drift; raw summary written to docs/evidence/k6-local-summary.json"
    exit 0
  fi
  sleep 0.5
done

echo "Count drift after outbox wait: Mongo=$actual Redis=${redis_count:-missing}" >&2
exit 1
