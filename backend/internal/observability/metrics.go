package observability

import (
	"sync/atomic"
	"time"
)

// Metrics contains dependency-free process counters suitable for a small
// deployment. Snapshot values can be scraped as JSON without adding a paid
// monitoring dependency.
type Metrics struct {
	startedAt          time.Time
	requests           atomic.Int64
	requestErrors      atomic.Int64
	requestDurationMic atomic.Int64
	websocketClients   atomic.Int64
	websocketRejected  atomic.Int64
	relayProcessed     atomic.Int64
	relayFailures      atomic.Int64
}

func NewMetrics() *Metrics { return &Metrics{startedAt: time.Now().UTC()} }

func (m *Metrics) ObserveRequest(status int, duration time.Duration) {
	m.requests.Add(1)
	if status >= 500 {
		m.requestErrors.Add(1)
	}
	m.requestDurationMic.Add(duration.Microseconds())
}

func (m *Metrics) TryOpenWebSocket(limit int64) bool {
	current := m.websocketClients.Add(1)
	if current <= limit {
		return true
	}
	m.websocketClients.Add(-1)
	m.websocketRejected.Add(1)
	return false
}

func (m *Metrics) CloseWebSocket() { m.websocketClients.Add(-1) }
func (m *Metrics) RelayProcessed() { m.relayProcessed.Add(1) }
func (m *Metrics) RelayFailed()    { m.relayFailures.Add(1) }

func (m *Metrics) Snapshot() map[string]any {
	requests := m.requests.Load()
	durationMic := m.requestDurationMic.Load()
	averageMs := float64(0)
	if requests > 0 {
		averageMs = float64(durationMic) / float64(requests) / 1000
	}
	return map[string]any{
		"uptimeSeconds":            int64(time.Since(m.startedAt).Seconds()),
		"httpRequestsTotal":        requests,
		"httpServerErrorsTotal":    m.requestErrors.Load(),
		"httpAverageDurationMs":    averageMs,
		"websocketClients":         m.websocketClients.Load(),
		"websocketRejectedTotal":   m.websocketRejected.Load(),
		"outboxProcessedTotal":     m.relayProcessed.Load(),
		"outboxRelayFailuresTotal": m.relayFailures.Load(),
	}
}
