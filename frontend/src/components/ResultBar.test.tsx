import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ResultBar } from './ResultBar'

const option = { id: 'durable', text: 'Durable votes', count: 3 }

describe('ResultBar', () => {
	it('keeps counts hidden before a voter makes a choice', () => {
		render(<ResultBar option={option} votes={3} total={5} voted={false} disabled={false} winner={true} reveal={false} onVote={() => {}} />)
		expect(screen.getByText('Durable votes')).toBeVisible()
		expect(screen.queryByText(/3 \(60%\)/)).not.toBeInTheDocument()
	})

	it('reveals an accessible result and forwards a vote', () => {
		const onVote = vi.fn()
		render(<ResultBar option={option} votes={3} total={5} voted disabled={false} winner reveal onVote={onVote} />)
		expect(screen.getByText(/3 \(60%\)/)).toBeVisible()
		fireEvent.click(screen.getByRole('button', { name: /Durable votes/ }))
		expect(onVote).toHaveBeenCalledWith(option)
	})
})
