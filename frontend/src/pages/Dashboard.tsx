import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiError } from '../lib/api'
import type { Poll } from '../lib/types'
import ShareCard from '../components/ShareCard'

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export default function Dashboard() {
  const [polls, setPolls] = useState<Poll[] | null>(null)
  const [error, setError] = useState('')
  const [shareSlug, setShareSlug] = useState<string | null>(null)

  if (polls === null && !error) {
    api
      .getMine()
      .then(setPolls)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Failed to load polls'))
  }

  async function toggleClose(p: Poll) {
    const res = await api.closePoll(p.slug).catch(() => null)
    if (!res) return
    setPolls((prev) =>
      prev ? prev.map((x) => (x.slug === p.slug ? { ...x, closed: res.closed } : x)) : prev,
    )
  }

  async function remove(p: Poll) {
    if (!confirm(`Delete "${p.title}"? This cannot be undone.`)) return
    await api.deletePoll(p.slug).catch(() => null)
    setPolls((prev) => (prev ? prev.filter((x) => x.slug !== p.slug) : prev))
  }

  return (
    <div className="page page-wide">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 style={{ letterSpacing: '-0.03em' }}>Your polls</h1>
        <Link to="/create" className="btn btn-primary">
          + New poll
        </Link>
      </div>

      {error && <p className="form-error">{error}</p>}

      {polls === null && !error && <div className="spinner" style={{ marginTop: 60 }} />}

      {polls && polls.length === 0 && (
        <div className="empty-state">
          <h2>No polls yet</h2>
          <p>Create one and start collecting votes in seconds.</p>
          <Link to="/create" className="btn btn-primary">
            Create your first poll
          </Link>
        </div>
      )}

      {polls?.map((p) => (
        <div className="card" key={p.slug}>
          <div className="poll-row">
            <div className="poll-row-head">
              <div>
                <h3 className="poll-row-title">{p.title}</h3>
                <p className="poll-row-sub">
                  {formatDate(p.created_at)} ·{' '}
                  <Link to={`/poll/${p.slug}`}>/poll/{p.slug}</Link>
                </p>
                <div className="poll-row-counts">
                  <span className="chip">{p.totalVotes} votes</span>
                  <span className="chip">
                    {p.closed ? 'Closed' : 'Live'}
                  </span>
                </div>
              </div>
              <div className="poll-row-actions">
                <button className="btn btn-ghost" onClick={() => setShareSlug(p.slug)}>
                  Share
                </button>
                <button className="btn btn-ghost" onClick={() => toggleClose(p)}>
                  {p.closed ? 'Reopen' : 'Close'}
                </button>
                <button className="btn btn-danger" onClick={() => remove(p)}>
                  Delete
                </button>
              </div>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
              {p.options.map((o) => (
                <span className="chip" key={o.id}>
                  {o.text} · {p.liveCounts?.[o.id] ?? o.count}
                </span>
              ))}
            </div>
          </div>
        </div>
      ))}

      {shareSlug && (
        <div className="modal-backdrop" onClick={() => setShareSlug(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-head">
              <h3>Share poll</h3>
              <button className="icon-btn" onClick={() => setShareSlug(null)} aria-label="Close">
                ×
              </button>
            </div>
            <ShareCard slug={shareSlug} />
          </div>
        </div>
      )}
    </div>
  )
}