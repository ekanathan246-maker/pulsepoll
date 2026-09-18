import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../lib/api'

export default function CreatePoll() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [options, setOptions] = useState(['', ''])
	const [closesAt, setClosesAt] = useState('')
	const [showResultsBeforeVote, setShowResultsBeforeVote] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  function changeOption(i: number, value: string) {
    setOptions((prev) => prev.map((o, idx) => (idx === i ? value : o)))
  }

  function addOption() {
	if (options.length >= 10) return
    setOptions((prev) => [...prev, ''])
  }

  function removeOption(i: number) {
    if (options.length <= 2) return
    setOptions((prev) => prev.filter((_, idx) => idx !== i))
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const poll = await api.createPoll({
        title,
        description,
        options: options.map((o) => o.trim()).filter(Boolean),
		closesAt: closesAt ? new Date(closesAt).toISOString() : undefined,
		showResultsBeforeVote,
      })
      navigate(`/poll/${poll.slug}`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create poll')
      setBusy(false)
    }
  }

  const filled = options.filter((o) => o.trim()).length

  return (
    <div className="page">
      <h1 style={{ letterSpacing: '-0.03em' }}>New poll</h1>
      <form onSubmit={submit}>
        <div className="card">
          <div className="form-row">
            <label htmlFor="title">Question</label>
            <input
              id="title"
              className="input"
              placeholder="e.g. Which framework should our team adopt?"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              minLength={3}
              maxLength={140}
            />
          </div>
          <div className="form-row">
            <label htmlFor="description">Description (optional)</label>
            <input
              id="description"
              className="input"
              placeholder="Give voters a little context…"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={500}
            />
          </div>
        </div>

        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <h2 className="card-title" style={{ marginBottom: 10 }}>Options</h2>
            <span className="char-hint">{filled} filled</span>
          </div>
          {options.map((opt, i) => (
            <div className="option-input-row" key={i}>
              <input
                className="input"
                placeholder={`Option ${i + 1}`}
                value={opt}
                onChange={(e) => changeOption(i, e.target.value)}
                maxLength={80}
                required={i < 2}
              />
              <button
                type="button"
                className="icon-btn"
                onClick={() => removeOption(i)}
                disabled={options.length <= 2}
                title="Remove option"
              >
                −
              </button>
            </div>
          ))}
		{options.length < 10 && (
            <button type="button" className="add-option" onClick={addOption}>
              + Add option
            </button>
          )}
        </div>

		<div className="card">
			<h2 className="card-title">Voting rules</h2>
			<p className="card-sub">Keep results suspenseful, and optionally let the poll close itself.</p>
			<div className="form-row">
				<label htmlFor="closesAt">Close automatically (optional)</label>
				<input id="closesAt" className="input" type="datetime-local" value={closesAt} onChange={(event) => setClosesAt(event.target.value)} />
			</div>
			<label className="check-row">
				<input type="checkbox" checked={showResultsBeforeVote} onChange={(event) => setShowResultsBeforeVote(event.target.checked)} />
				<span>Show live results before someone votes</span>
			</label>
		</div>

        {error && <p className="form-error">{error}</p>}
        <button className="btn btn-primary btn-block" disabled={busy || filled < 2}>
          {busy ? 'Creating…' : 'Create poll'}
        </button>
      </form>
    </div>
  )
}
