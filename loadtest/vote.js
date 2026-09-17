import http from 'k6/http'
import { check } from 'k6'

export const options = {
  scenarios: {
    vote_burst: {
      executor: 'constant-arrival-rate',
      duration: __ENV.DURATION || '20s',
      rate: Number(__ENV.RATE || 20),
      timeUnit: '1s',
      preAllocatedVUs: 25,
      maxVUs: 100,
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
  const voterID = `k6-${__VU}-${__ITER}-${Date.now()}-${Math.random().toString(36).slice(2)}`
  const response = http.post(
    `${baseURL}/api/polls/${slug}/vote`,
    JSON.stringify({ optionId: optionID }),
    {
      headers: {
        'Content-Type': 'application/json',
        // Every iteration models a new browser identity. The application is
        // explicit that this cookie is abuse friction, not proof of a person.
        Cookie: `ppv=${voterID}`,
        'User-Agent': `pulsepoll-k6-vu-${__VU}`,
      },
    },
  )
  check(response, { 'vote accepted': (res) => res.status === 202 })
}
