import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, ApiError, streamPoll } from '../lib/api'
import type { Poll, VoteUpdate } from '../lib/types'
import { ResultBar, usePollStats } from '../components/ResultBar'
import ShareCard from '../components/ShareCard'

const VOTED_KEY = (slug: string) => `ppvoted:${slug}`

export default function PollView() {
  const { slug = '' } = useParams()
  const [poll, setPoll] = useState<Poll | null>(null)
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [notFound, setNotFound] = useState(false)
  const [votedOption, setVotedOption] = useState<string | null>(() =>
    localStorage.getItem(VOTED_KEY(slug)),
  )
  const [error, setError] = useState('')
	const [connection, setConnection] = useState<'connecting' | 'live' | 'reconnecting'>('connecting')
	const versionRef = useRef(0)

  const { total: totalVotes, winners } = usePollStats(counts)
  const isClosed = poll?.closed ?? false

  const syncPoll = useCallback(() => {
    return api
      .getPoll(slug)
      .then((p) => {
        setPoll(p)
				versionRef.current = p.version
        setCounts(p.liveCounts ?? {})
        if (p.closed) {
          setCounts(
            Object.fromEntries(
              p.options.map((o: { id: string; count: number }) => [o.id, o.count]),
            ) as Record<string, number>,
          )
        }
      })
      .catch(() => setNotFound(true))
  }, [slug])

  // Load durable state, then keep a versioned WebSocket connected. A gap or
  // reconnect always replaces local state from the REST snapshot.
  useEffect(() => {
		let socket: WebSocket | null = null
		let retryTimer: number | undefined
		let stopped = false
		let attempt = 0
		void syncPoll()

		function connect() {
			if (stopped) return
			setConnection(attempt === 0 ? 'connecting' : 'reconnecting')
			socket = streamPoll(slug)
			socket.onopen = () => {
				attempt = 0
				setConnection('live')
				void syncPoll()
			}
			socket.onmessage = (event) => {
				try {
					const data = JSON.parse(event.data) as VoteUpdate
					if (data.type !== 'snapshot' && data.version !== versionRef.current + 1) {
						void syncPoll()
						return
					}
					versionRef.current = data.version
					if (!data.counts) {
						versionRef.current = data.version
						return
					}
					setCounts(data.counts)
					const total = Object.values(data.counts).reduce((sum, count) => sum + count, 0)
					setPoll((prev) => prev ? {
						...prev, version: data.version, closed: data.status === 'closed', totalVotes: total,
					} : prev)
				} catch {
					void syncPoll()
				}
			}
			socket.onclose = () => {
				if (stopped) return
				setConnection('reconnecting')
				attempt += 1
				const delay = Math.min(10_000, 500 * 2 ** Math.min(attempt, 5)) + Math.random() * 300
				retryTimer = window.setTimeout(connect, delay)
			}
			socket.onerror = () => socket?.close()
		}
		connect()
		return () => {
			stopped = true
			if (retryTimer) window.clearTimeout(retryTimer)
			socket?.close()
		}
	}, [slug, syncPoll, votedOption])

  async function castVote(optionId: string) {
    if (votedOption || isClosed) return
    setError('')
    try {
      const res = await api.vote(slug, optionId)
      localStorage.setItem(VOTED_KEY(slug), optionId)
      setVotedOption(optionId)
      setCounts(res.counts)
		setPoll((prev) => (prev ? { ...prev, totalVotes: res.total, version: res.version, canViewResults: true } : prev))
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // Already voted on this device – reconcile with server truth.
        syncPoll()
      }
      setError(err instanceof ApiError ? err.message : 'Vote failed')
    }
  }

  if (notFound) {
    return (
      <div className="page">
        <div className="empty-state">
          <h2>Poll not found</h2>
          <p>That link doesn’t point to any poll.</p>
        </div>
      </div>
    )
  }

  if (!poll) {
    return (
      <div className="center-page">
        <div className="spinner" />
      </div>
    )
  }

  const showLive = !isClosed && totalVotes >= 0

  return (
    <div className="page">
      <div className="poll-header">
        <div className="poll-status-row">
          {isClosed ? (
            <span className="live-pill closed-pill">
              <span className="dot" /> Closed
            </span>
          ) : (
			<span className={`live-pill${connection === 'live' ? '' : ' syncing-pill'}`}>
				<span className="dot" /> {connection === 'live' ? 'Live · synced' : connection === 'reconnecting' ? 'Reconnecting' : 'Connecting'}
            </span>
          )}
          <span className="total-votes">
            <strong>{totalVotes}</strong> {totalVotes === 1 ? 'vote' : 'votes'}
          </span>
        </div>
        <h1 style={{ fontSize: 'clamp(26px, 4vw, 38px)', margin: '0 0 6px', letterSpacing: '-0.03em' }}>
          {poll.title}
        </h1>
        {poll.description && <p style={{ color: 'var(--muted)', margin: 0 }}>{poll.description}</p>}
      </div>

      <div className="card">
        <div className="results">
          {poll.options.map((o) => {
            const votes = counts[o.id] ?? o.count ?? 0
            return (
              <ResultBar
                key={o.id}
                option={o}
                votes={votes}
                total={totalVotes}
                voted={votedOption === o.id}
                disabled={isClosed || !!votedOption}
                winner={winners.has(o.id)}
				reveal={poll.canViewResults || !!votedOption || isClosed}
                onVote={(opt) => castVote(opt.id)}
              />
            )
          })}
        </div>

        {showLive && (
          <p className="card-sub" style={{ marginTop: 16, marginBottom: 0 }}>
            {votedOption
              ? 'Results update live as votes come in.'
              : 'Tap an option to cast your vote. Updates stream in live.'}
          </p>
        )}
        {isClosed && (
          <p className="card-sub" style={{ marginTop: 16, marginBottom: 0 }}>
            This poll is closed — results are final.
          </p>
        )}
      </div>

      <ShareCard slug={slug} />

      {error && <p className="form-error" style={{ marginTop: 14 }}>{error}</p>}
    </div>
  )
}
