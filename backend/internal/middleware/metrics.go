package middleware

import (
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector tracks HTTP request metrics in-memory.
// A background collector goroutine calls Snapshot() each interval to read and reset.
type MetricsCollector struct {
	mu        sync.Mutex
	latencies []float64

	requestCount atomic.Int64
	status2xx    atomic.Int64
	status3xx    atomic.Int64
	status4xx    atomic.Int64
	status5xx    atomic.Int64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		latencies: make([]float64, 0, 1024),
	}
}

// HTTPMetricsSnapshot holds a point-in-time snapshot of collected HTTP metrics.
type HTTPMetricsSnapshot struct {
	RequestCount int64
	LatencyP50   float64
	LatencyP95   float64
	LatencyP99   float64
	Status2xx    int64
	Status3xx    int64
	Status4xx    int64
	Status5xx    int64
}

// Snapshot returns the current metrics and resets all counters.
func (mc *MetricsCollector) Snapshot() HTTPMetricsSnapshot {
	snap := HTTPMetricsSnapshot{
		RequestCount: mc.requestCount.Swap(0),
		Status2xx:    mc.status2xx.Swap(0),
		Status3xx:    mc.status3xx.Swap(0),
		Status4xx:    mc.status4xx.Swap(0),
		Status5xx:    mc.status5xx.Swap(0),
	}

	mc.mu.Lock()
	lats := mc.latencies
	mc.latencies = make([]float64, 0, 1024)
	mc.mu.Unlock()

	if len(lats) > 0 {
		sort.Float64s(lats)
		snap.LatencyP50 = percentile(lats, 50)
		snap.LatencyP95 = percentile(lats, 95)
		snap.LatencyP99 = percentile(lats, 99)
	}
	return snap
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// metricsResponseWriter wraps http.ResponseWriter to capture the status code.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware returns an http.Handler that records request metrics.
func (mc *MetricsCollector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &metricsResponseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(rw, r)

		elapsed := float64(time.Since(start).Milliseconds())

		mc.requestCount.Add(1)
		switch {
		case rw.statusCode >= 500:
			mc.status5xx.Add(1)
		case rw.statusCode >= 400:
			mc.status4xx.Add(1)
		case rw.statusCode >= 300:
			mc.status3xx.Add(1)
		default:
			mc.status2xx.Add(1)
		}

		mc.mu.Lock()
		if len(mc.latencies) < 10000 { // cap to prevent OOM between snapshots
			mc.latencies = append(mc.latencies, elapsed)
		}
		mc.mu.Unlock()
	})
}
