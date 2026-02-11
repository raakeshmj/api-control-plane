package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// MetricsCollector tracks request counts and latency distribution
type MetricsCollector struct {
	TotalRequests uint64
	TotalErrors   uint64
	StatusCounts  map[int]uint64
	ClientUsage   map[string]uint64

	// Latency Reservoir
	latencies  []time.Duration
	maxSamples int
	mu         sync.RWMutex
}

func NewCollector(maxSamples int) *MetricsCollector {
	return &MetricsCollector{
		StatusCounts: make(map[int]uint64),
		latencies:    make([]time.Duration, 0, maxSamples),
		maxSamples:   maxSamples,
	}
}

func (c *MetricsCollector) Record(duration time.Duration, statusCode int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.TotalRequests++
	if statusCode >= 400 {
		c.TotalErrors++
	}
	c.StatusCounts[statusCode]++

	// Reservoir Sampling (Simple Ring Buffer for now, or just append until full then random replace)
	if len(c.latencies) < c.maxSamples {
		c.latencies = append(c.latencies, duration)
	} else {
		// Replace random? Or circular? Circular is bias for "recent" which is good.
		// Let's just overwrite circle (ignoring true uniform sampling for simplicity)
		// Actually, let's just keep last N samples (Sliding Window).
		c.latencies = c.latencies[1:]
		c.latencies = append(c.latencies, duration)
	}
}

// Snapshot returns calculated stats
type Stats struct {
	TotalRequests uint64         `json:"total_requests"`
	TotalErrors   uint64         `json:"total_errors"`
	ErrorRate     float64        `json:"error_rate"`
	P50Latency    string         `json:"p50_latency"`
	P95Latency    string         `json:"p95_latency"`
	P99Latency    string         `json:"p99_latency"`
	StatusCounts  map[int]uint64 `json:"status_counts"`
}

func (c *MetricsCollector) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Copy latencies to sort for quantiles
	sorted := make([]time.Duration, len(c.latencies))
	copy(sorted, c.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p50 := time.Duration(0)
	p95 := time.Duration(0)
	p99 := time.Duration(0)

	if len(sorted) > 0 {
		p50 = sorted[int(math.Max(0, float64(len(sorted))*0.50-1))] // -1 for index
		p95 = sorted[int(math.Max(0, float64(len(sorted))*0.95-1))]
		p99 = sorted[int(math.Max(0, float64(len(sorted))*0.99-1))] // Should round?

		// Index logic:
		// N=100. p50 -> index 49 or 50.
		// ceil(N * P) - 1 ?
		// int(N * P). If N=100, P=0.5 -> 50. Index 50 is 51st element. OK.
		// Let's use simple int casting.
		p50 = sorted[int(float64(len(sorted))*0.50)]
		p95 = sorted[int(float64(len(sorted))*0.95)]
		// For 99, safeguard bounds
		idx99 := int(float64(len(sorted)) * 0.99)
		if idx99 >= len(sorted) {
			idx99 = len(sorted) - 1
		}
		p99 = sorted[idx99]
	}

	errorRate := 0.0
	if c.TotalRequests > 0 {
		errorRate = float64(c.TotalErrors) / float64(c.TotalRequests)
	}

	// Copy map
	sc := make(map[int]uint64)
	for k, v := range c.StatusCounts {
		sc[k] = v
	}

	return Stats{
		TotalRequests: c.TotalRequests,
		TotalErrors:   c.TotalErrors,
		ErrorRate:     errorRate,
		P50Latency:    p50.String(),
		P95Latency:    p95.String(),
		P99Latency:    p99.String(),
		StatusCounts:  sc,
	}
}
