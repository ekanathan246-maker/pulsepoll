package observability

import (
	"testing"
	"time"
)

func TestMetricsTrackRequestsAndBoundWebSockets(t *testing.T) {
	metrics := NewMetrics()
	metrics.ObserveRequest(200, 10*time.Millisecond)
	metrics.ObserveRequest(503, 30*time.Millisecond)

	if !metrics.TryOpenWebSocket(1) {
		t.Fatal("first WebSocket should be accepted")
	}
	if metrics.TryOpenWebSocket(1) {
		t.Fatal("connection above the cap should be rejected")
	}
	metrics.CloseWebSocket()
	metrics.RelayProcessed()
	metrics.RelayFailed()

	snapshot := metrics.Snapshot()
	assertMetric(t, snapshot, "httpRequestsTotal", int64(2))
	assertMetric(t, snapshot, "httpServerErrorsTotal", int64(1))
	assertMetric(t, snapshot, "websocketClients", int64(0))
	assertMetric(t, snapshot, "websocketRejectedTotal", int64(1))
	assertMetric(t, snapshot, "outboxProcessedTotal", int64(1))
	assertMetric(t, snapshot, "outboxRelayFailuresTotal", int64(1))
	if got := snapshot["httpAverageDurationMs"].(float64); got != 20 {
		t.Fatalf("average duration = %v, want 20", got)
	}
}

func assertMetric(t *testing.T, snapshot map[string]any, name string, want int64) {
	t.Helper()
	if got := snapshot[name]; got != want {
		t.Fatalf("%s = %v, want %d", name, got, want)
	}
}
