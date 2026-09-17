import http from 'k6/http'
import { check } from 'k6'
import ws from 'k6/ws'
import { Counter, Rate, Trend } from 'k6/metrics'

const votesAccepted = new Counter('votes_accepted')
const voteFailures = new Rate('vote_failures')
const voteDuration = new Trend('vote_duration', true)
const websocketFailures = new Rate('websocket_failures')

export const options = {
  scenarios: {
    vote_burst: {
      executor: 'constant-arrival-rate',
      exec: 'voteBurst',
      duration: __ENV.DURATION || '20s',
      rate: Number(__ENV.RATE || 20),
      timeUnit: '1s',
      preAllocatedVUs: 25,
      maxVUs: 100,
    },
    reconnecting_viewers: {
      executor: 'constant-arrival-rate',
      exec: 'reconnectViewer',
      rate: Number(__ENV.VIEWER_RATE || 5),
      timeUnit: '1s',
      duration: __ENV.DURATION || '20s',
      startTime: '1s',
      preAllocatedVUs: 5,
      maxVUs: 20,
      gracefulStop: '5s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    vote_failures: ['rate<0.01'],
    vote_duration: ['p(95)<400'],
    websocket_failures: ['rate<0.01'],
  },
}

const baseURL = __ENV.BASE_URL || 'http://localhost'
const slug = __ENV.POLL_SLUG
const optionID = __ENV.OPTION_ID

export function voteBurst() {
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
  const accepted = check(response, { 'vote accepted': (res) => res.status === 202 })
  voteFailures.add(!accepted)
  voteDuration.add(response.timings.duration)
  if (accepted) votesAccepted.add(1)
}

export function reconnectViewer() {
  if (!slug) throw new Error('Set POLL_SLUG')
  const wsURL = `${baseURL.replace(/^http/, 'ws')}/api/polls/${slug}/live`
  const response = ws.connect(wsURL, {}, (socket) => {
    socket.on('message', () => socket.close())
    socket.setTimeout(() => socket.close(), 2500)
  })
  const upgraded = check(response, { 'viewer connected and received snapshot': (res) => res && res.status === 101 })
  websocketFailures.add(!upgraded)
}
