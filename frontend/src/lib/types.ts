export interface User {
  id: string
  name: string
  email: string
}

export interface Option {
  id: string
  text: string
  count: number
}

export interface Poll {
  id: string
  slug: string
  title: string
  description: string
  options: Option[]
  created_by: string
  created_at: string
  closed: boolean
  hide_results: boolean
	version: number
	closes_at?: string
  totalVotes: number
  liveCounts: Record<string, number>
	canViewResults: boolean
}

export interface VoteUpdate {
	type: 'snapshot' | 'vote.applied' | 'poll.closed' | 'poll.reopened'
	eventId?: string
	pollId: string
	version: number
	counts?: Record<string, number>
  total: number
	status: 'open' | 'closed'
}
