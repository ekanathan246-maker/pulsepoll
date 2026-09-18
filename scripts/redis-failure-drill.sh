#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost}"
COOKIE_FILE="$(mktemp -t pulsepoll-drill-cookies.XXXXXX)"
BODY_FILE="$(mktemp -t pulsepoll-drill-body.XXXXXX)"
trap 'rm -f "$COOKIE_FILE" "$BODY_FILE"' EXIT

email="redis-drill-$(date +%s)-$RANDOM@example.test"
signup="$(jq -nc --arg email "$email" '{name:"Redis Failure Drill",email:$email,password:"FailureDrill!2026"}')"
status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -c "$COOKIE_FILE" -H 'Content-Type: application/json' --data "$signup" "$BASE_URL/api/auth/signup")"
[[ "$status" == "200" ]] || { echo "Signup failed ($status)" >&2; exit 1; }
csrf="$(awk '$6 == "pp_csrf" { value=$7 } END { print value }' "$COOKIE_FILE")"

poll_payload='{"title":"Redis outage must not lose this vote","options":["Durable","Lost"],"showResultsBeforeVote":true}'
status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -b "$COOKIE_FILE" -c "$COOKIE_FILE" -H 'Content-Type: application/json' -H "X-CSRF-Token: $csrf" --data "$poll_payload" "$BASE_URL/api/polls")"
[[ "$status" == "201" ]] || { echo "Poll creation failed ($status): $(cat "$BODY_FILE")" >&2; exit 1; }
slug="$(jq -r '.slug' "$BODY_FILE")"
option_id="$(jq -r '.options[0].id' "$BODY_FILE")"

docker compose stop redis >/dev/null
vote_payload="$(jq -nc --arg optionId "$option_id" '{optionId:$optionId}')"
status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -b "$COOKIE_FILE" -c "$COOKIE_FILE" -H 'Content-Type: application/json' --data "$vote_payload" "$BASE_URL/api/polls/$slug/vote")"
if [[ "$status" != "202" ]]; then
  docker compose start redis >/dev/null
  echo "Vote was not durably accepted during Redis outage ($status): $(cat "$BODY_FILE")" >&2
  exit 1
fi

docker compose start redis >/dev/null
for _ in $(seq 1 40); do
  redis_version="$(docker compose exec -T redis redis-cli GET "pp:poll:$slug:version" 2>/dev/null || true)"
  redis_count="$(docker compose exec -T redis redis-cli HGET "pp:poll:$slug:counts" "$option_id" 2>/dev/null || true)"
  if [[ "$redis_version" == "2" && "$redis_count" == "1" ]]; then
    snapshot="$(curl -fsS "$BASE_URL/api/polls/$slug")"
    [[ "$(jq -r '.version' <<<"$snapshot")" == "2" ]]
    [[ "$(jq -r '.totalVotes' <<<"$snapshot")" == "1" ]]
    echo "PASS: vote accepted with Redis offline; outbox repaired Redis to version=2 count=1."
    exit 0
  fi
  sleep 0.5
done

echo "Redis did not catch up after restart" >&2
exit 1

