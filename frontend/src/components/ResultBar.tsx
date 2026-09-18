import { useMemo } from 'react'
import type { Option } from '../lib/types'

interface ResultBarProps {
  option: Option
  votes: number
  total: number
  voted: boolean
  disabled: boolean
  winner: boolean
	reveal: boolean
  onVote: (option: Option) => void
}

export function ResultBar({ option, votes, total, voted, disabled, winner, reveal, onVote }: ResultBarProps) {
  const pct = total > 0 ? Math.round((votes / total) * 100) : 0
  return (
    <button
      className={`result-option${voted ? ' voted' : ''}${winner ? ' winner' : ''}`}
      onClick={() => onVote(option)}
      disabled={disabled}
      title={disabled ? undefined : option.text}
    >
		{reveal && <div className="bar-track">
        <div className="bar-fill" style={{ width: `${pct}%` }} />
		</div>}
      <div className="option-top">
        <span className="option-text">{option.text}</span>
		{reveal && <span className="option-meta">
          {voted && <span className="voted-badge">Your vote · </span>}
          {votes} ({pct}%)
		</span>}
      </div>
    </button>
  )
}

export function usePollStats(counts: Record<string, number>) {
  return useMemo(() => {
    const total = Object.values(counts).reduce((a, b) => a + b, 0)
    const max = total > 0 ? Math.max(...Object.values(counts)) : 0
    const winners = new Set(
      Object.entries(counts)
        .filter(([, v]) => v === max && max > 0)
        .map(([k]) => k),
    )
    return { total, winners }
  }, [counts])
}
