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
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)

  const { total: totalVotes, winners } = usePollStats(counts)
  const isClosed = poll?.closed ?? false

  const syncPoll = useCallback(() => {
    return api
      .getPoll(slug)
      .then((p) => {
        setPoll(p)
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

  // Load poll + open the SSE stream
  useEffect(() => {
    syncPoll()

    const es = streamPoll(slug)
    esRef.current = es
    es.onopen = () => {
      setConnected(true)
      // On (re)connect, re-fetch to get the freshest snapshot.
      syncPoll()
    }
    es.addEventListener('update', (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data) as VoteUpdate
        if (data.closed !== undefined) {
          setPoll((prev) => (prev ? { ...prev, closed: Boolean(data.closed) } : prev))
        }
        if (data.counts) {
          setCounts(data.counts)
          setPoll((prev) => (prev ? { ...prev, totalVotes: data.total } : prev))
        }
      } catch {
        /* ignore malformed frame */
      }
    })
    es.onerror = () => setConnected(false)

    return () => es.close()
  }, [slug, syncPoll])

  async function castVote(optionId: string) {
    if (votedOption || isClosed) return
    setError('')
    try {
      const res = await api.vote(slug, optionId)
      localStorage.setItem(VOTED_KEY(slug), optionId)
      setVotedOption(optionId)
      setCounts(res.counts)
      setPoll((prev) => (prev ? { ...prev, totalVotes: res.total } : prev))
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
            <span className="live-pill">
              <span className="dot" /> Live{connected ? '' : ' · reconnecting'}
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