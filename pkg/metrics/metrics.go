package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TradesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trades_processed_total",
		Help: "Total number of trades processed",
	}, []string{"status"})

	TradesProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "trades_processing_duration_seconds",
		Help:    "Duration of trade processing",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	CacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total number of cache hits",
	})

	CacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total number of cache misses",
	})

	DatabaseQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "database_queries_total",
		Help: "Total number of database queries",
	}, []string{"query_type", "status"})

	DatabaseQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "database_query_duration_seconds",
		Help:    "Duration of database queries",
		Buckets: prometheus.DefBuckets,
	}, []string{"query_type"})

	AggregationRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aggregation_requests_total",
		Help: "Total number of aggregation requests",
	}, []string{"ticker", "cached"})

	ActiveGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_goroutines",
		Help: "Number of active goroutines",
	})

	MemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})
)

func RecordCacheHit() {
	CacheHits.Inc()
}

func RecordCacheMiss() {
	CacheMisses.Inc()
}

func RecordDatabaseQuery(queryType, status string, duration float64) {
	DatabaseQueries.WithLabelValues(queryType, status).Inc()
	DatabaseQueryDuration.WithLabelValues(queryType).Observe(duration)
}

func RecordTradeProcessed(status string) {
	TradesProcessed.WithLabelValues(status).Inc()
}

func RecordAggregationRequest(ticker string, cached bool) {
	cachedStr := "false"
	if cached {
		cachedStr = "true"
	}
	AggregationRequests.WithLabelValues(ticker, cachedStr).Inc()
}

type Timer struct {
	start time.Time
}

func NewTimer() *Timer {
	return &Timer{
		start: time.Now(),
	}
}

func (t *Timer) ObserveDuration(observer prometheus.Observer) {
	observer.Observe(time.Since(t.start).Seconds())
}

func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}
