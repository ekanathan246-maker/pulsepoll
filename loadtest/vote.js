import http from 'k6/http'
import { check, sleep } from 'k6'

export const options = {
  scenarios: {
    vote_burst: {
      executor: 'ramping-vus',
      stages: [
        { duration: '10s', target: 25 },
        { duration: '20s', target: 50 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<400'],
  },
}

const baseURL = __ENV.BASE_URL || 'http://localhost'
const slug = __ENV.POLL_SLUG
const optionID = __ENV.OPTION_ID

export default function () {
  if (!slug || !optionID) throw new Error('Set POLL_SLUG and OPTION_ID')
  const response = http.post(
    `${baseURL}/api/polls/${slug}/vote`,
    JSON.stringify({ optionId: optionID }),
    {
      headers: {
        'Content-Type': 'application/json',
        // Give each virtual user a stable browser identity. The server stores
        // the returned ppv cookie in k6's per-VU cookie jar.
        'User-Agent': `pulsepoll-k6-vu-${__VU}`,
      },
    },
  )
  check(response, { 'vote accepted or duplicate': (res) => res.status === 202 || res.status === 409 })
  sleep(0.2)
}
