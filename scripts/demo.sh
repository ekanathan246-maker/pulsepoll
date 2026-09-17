#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-seed}"
BASE_URL="${2:-http://localhost}"
DEMO_EMAIL="${DEMO_EMAIL:-interview-demo-2026@pulsepoll.local}"
DEMO_PASSWORD="${DEMO_PASSWORD:-PulsePollDemo!2026}"

for command in curl jq awk mktemp; do
  command -v "$command" >/dev/null || { echo "Missing required command: $command" >&2; exit 1; }
done

case "$BASE_URL" in
  http://localhost*|http://127.0.0.1*) ;;
  *)
    if [[ "${ALLOW_REMOTE_DEMO:-}" != "1" || "$DEMO_PASSWORD" == "PulsePollDemo!2026" ]]; then
      echo "Remote demo seeding is disabled by default. Set ALLOW_REMOTE_DEMO=1 and a unique DEMO_PASSWORD." >&2
      exit 1
    fi
    ;;
esac

COOKIE_FILE="$(mktemp -t pulsepoll-demo-cookies.XXXXXX)"
BODY_FILE="$(mktemp -t pulsepoll-demo-body.XXXXXX)"
trap 'rm -f "$COOKIE_FILE" "$BODY_FILE"' EXIT

json_request() {
  local method="$1" path="$2" payload="${3:-}" csrf="${4:-}"
  local args=(-sS -o "$BODY_FILE" -w '%{http_code}' -X "$method" -b "$COOKIE_FILE" -c "$COOKIE_FILE")
  [[ -n "$payload" ]] && args+=(-H 'Content-Type: application/json' --data "$payload")
  [[ -n "$csrf" ]] && args+=(-H "X-CSRF-Token: $csrf")
  curl "${args[@]}" "$BASE_URL$path"
}

login() {
  local signup_payload login_payload status
  signup_payload="$(jq -nc --arg name 'PulsePoll Demo Host' --arg email "$DEMO_EMAIL" --arg password "$DEMO_PASSWORD" '{name:$name,email:$email,password:$password}')"
  status="$(json_request POST /api/auth/signup "$signup_payload")"
  if [[ "$status" == "409" ]]; then
    login_payload="$(jq -nc --arg email "$DEMO_EMAIL" --arg password "$DEMO_PASSWORD" '{email:$email,password:$password}')"
    status="$(json_request POST /api/auth/login "$login_payload")"
  fi
  [[ "$status" == "200" ]] || { echo "Demo login failed ($status): $(cat "$BODY_FILE")" >&2; exit 1; }
}

csrf_token() {
  awk '$6 == "pp_csrf" { value=$7 } END { print value }' "$COOKIE_FILE"
}

login
CSRF="$(csrf_token)"
[[ -n "$CSRF" ]] || { echo "CSRF cookie was not returned" >&2; exit 1; }

if [[ "$MODE" == "reset" ]]; then
  status="$(json_request GET /api/polls)"
  [[ "$status" == "200" ]] || { echo "Could not list demo polls ($status)" >&2; exit 1; }
  count=0
  while IFS= read -r slug; do
    [[ -n "$slug" ]] || continue
    status="$(json_request DELETE "/api/polls/$slug" '' "$CSRF")"
    [[ "$status" == "200" ]] || { echo "Could not archive demo poll $slug ($status)" >&2; exit 1; }
    count=$((count + 1))
  done < <(jq -r '.[] | select(.title | startswith("[DEMO]")) | .slug' "$BODY_FILE")
  echo "Archived $count demo poll(s)."
  exit 0
fi

[[ "$MODE" == "seed" ]] || { echo "Usage: scripts/demo.sh [seed|reset] [base-url]" >&2; exit 1; }
payload="$(jq -nc '{title:"[DEMO] What proves production engineering?",description:"Vote from a second browser and watch both screens update live.",options:["Durable correctness","Only visual polish","More features"],showResultsBeforeVote:false}')"
status="$(json_request POST /api/polls "$payload" "$CSRF")"
[[ "$status" == "201" ]] || { echo "Could not create demo poll ($status): $(cat "$BODY_FILE")" >&2; exit 1; }
slug="$(jq -r '.slug' "$BODY_FILE")"
echo "Seeded demo poll: $BASE_URL/poll/$slug"
echo "Reset later with: scripts/demo.sh reset $BASE_URL"
