#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-${BASE_URL:-}}"
[[ -n "$BASE_URL" ]] || { echo "Usage: $0 https://your-pulsepoll.example" >&2; exit 2; }
BASE_URL="${BASE_URL%/}"

if [[ "$BASE_URL" != https://* && "${ALLOW_HTTP:-}" != "1" ]]; then
  echo "Refusing a non-HTTPS public verification. Set ALLOW_HTTP=1 only for local testing." >&2
  exit 2
fi

live="$(curl --fail --silent --show-error --max-time 75 "$BASE_URL/api/livez")"
ready="$(curl --fail --silent --show-error --max-time 10 "$BASE_URL/api/readyz")"
config="$(curl --fail --silent --show-error --max-time 10 "$BASE_URL/api/config")"
landing="$(curl --fail --silent --show-error --max-time 10 "$BASE_URL/")"

[[ "$(jq -r '.status' <<<"$live")" == "ok" ]] || { echo "Unexpected liveness response: $live" >&2; exit 1; }
[[ "$(jq -r '.status' <<<"$ready")" == "ready" ]] || { echo "Unexpected readiness response: $ready" >&2; exit 1; }
[[ "$(jq -r '.publicUrl' <<<"$config")" == "$BASE_URL" ]] || { echo "PUBLIC_URL does not match $BASE_URL: $config" >&2; exit 1; }
grep -q 'PulsePoll' <<<"$landing" || { echo "Landing page does not contain PulsePoll" >&2; exit 1; }

echo "PASS: $BASE_URL serves PulsePoll and reports live, ready, and the exact public URL."
