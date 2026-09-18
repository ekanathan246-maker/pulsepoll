import type { Poll, User } from './types'

// Public base URL for share links, fetched from the backend so it can be
// changed at runtime without rebuilding the frontend.
let publicUrl = ''

export async function loadConfig(): Promise<void> {
  try {
    const cfg = await request<{ publicUrl: string }>('/config')
    publicUrl = cfg.publicUrl?.trim() || ''
  } catch {
    publicUrl = ''
  }
}

// shareUrl builds a link people can actually open — the configured public
// URL when set, otherwise whatever origin served the page.
export function shareUrl(slug: string): string {
  const base = publicUrl || window.location.origin
  return `${base}/poll/${slug}`
}

// Small fetch wrapper. Cookies are same-origin (dev proxy / prod nginx) so
// we always send credentials.
export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const method = (init?.method ?? 'GET').toUpperCase()
	const csrfToken = document.cookie
		.split('; ')
		.find((entry) => entry.startsWith('pp_csrf='))
		?.slice('pp_csrf='.length)
  const res = await fetch(`/api${path}`, {
		...init,
    credentials: 'include',
    headers: {
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
			...(method !== 'GET' && method !== 'HEAD' && csrfToken ? { 'X-CSRF-Token': decodeURIComponent(csrfToken) } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    let msg = `${res.status}`
    try {
      const body = await res.json()
			if (typeof body?.error === 'string') msg = body.error
			if (body?.error?.message) msg = body.error.message
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  me: () => request<User>('/auth/me'),
  login: (email: string, password: string) =>
    request<User>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  signup: (name: string, email: string, password: string) =>
    request<User>('/auth/signup', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),
  logout: () => request<{ message: string }>('/auth/logout', { method: 'POST' }),

	createPoll: (body: { title: string; description: string; options: string[]; closesAt?: string; showResultsBeforeVote: boolean }) =>
    request<Poll>('/polls', { method: 'POST', body: JSON.stringify(body) }),

  getMine: () => request<Poll[]>('/polls'),

  getPoll: (slug: string) => request<Poll>(`/polls/${slug}`),

  vote: (slug: string, optionId: string) =>
	request<{ accepted: boolean; version: number; counts: Record<string, number>; total: number }>(`/polls/${slug}/vote`, {
      method: 'POST',
      body: JSON.stringify({ optionId }),
    }),

  closePoll: (slug: string) =>
    request<{ closed: boolean }>(`/polls/${slug}/close`, { method: 'PATCH' }),

  deletePoll: (slug: string) =>
    request<{ message: string }>(`/polls/${slug}`, { method: 'DELETE' }),
}

export function streamPoll(slug: string): WebSocket {
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
	return new WebSocket(`${protocol}//${window.location.host}/api/polls/${slug}/live`)
}
