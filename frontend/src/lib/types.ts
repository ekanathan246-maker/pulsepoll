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
  totalVotes: number
  liveCounts: Record<string, number>
}

export interface VoteUpdate {
  counts: Record<string, number>
  total: number
  closed?: boolean
}