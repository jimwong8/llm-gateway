package longcontext

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	tasksCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_tasks_created_total",
		Help: "Total number of long-context tasks created",
	})

	tasksSucceeded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_tasks_succeeded_total",
		Help: "Total number of long-context tasks that succeeded",
	})

	tasksFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "longcontext_tasks_failed_total",
		Help: "Total number of long-context tasks that failed",
	}, []string{"reason"})

	phaseDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "longcontext_phase_duration_seconds",
		Help:    "Duration of each task phase in seconds",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"phase"})

	chunksProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_chunks_processed_total",
		Help: "Total number of chunks processed",
	})

	chunksFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_chunks_failed_total",
		Help: "Total number of chunk processing failures",
	})

	chunkMapDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "longcontext_chunk_map_duration_seconds",
		Help:    "Duration of chunk map phase in seconds",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
	})

	evidenceGenerated = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "longcontext_evidence_count",
		Help:    "Number of evidence items generated per task",
		Buckets: []float64{0, 1, 5, 10, 20, 50, 100},
	})

	citationCoverage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "longcontext_citation_coverage",
		Help:    "Citation coverage ratio per task",
		Buckets: []float64{0, 0.1, 0.2, 0.3, 0.5, 0.8, 1.0},
	})

	activeWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_active_workers",
		Help: "Number of currently active long-context workers",
	})

	workerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_worker_pool_size",
		Help: "Configured long-context worker pool size",
	})

	queueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_queue_depth",
		Help: "Number of tasks waiting in queue (status=queued)",
	})

	verificationFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_verification_failures_total",
		Help: "Total number of verification failures",
	})

	tokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "longcontext_tokens_used_total",
		Help: "Total tokens used by long-context tasks",
	}, []string{"phase"})
)

func RecordTaskCreated()             { tasksCreated.Inc() }
func RecordTaskSucceeded()           { tasksSucceeded.Inc() }
func RecordTaskFailed(reason string) { tasksFailed.WithLabelValues(reason).Inc() }
func ObservePhaseDuration(phase string, d time.Duration) {
	phaseDuration.WithLabelValues(phase).Observe(d.Seconds())
}
func RecordChunkProcessed()                   { chunksProcessed.Inc() }
func RecordChunkFailed()                      { chunksFailed.Inc() }
func ObserveChunkMapDuration(d time.Duration) { chunkMapDuration.Observe(d.Seconds()) }
func ObserveEvidenceCount(n int)              { evidenceGenerated.Observe(float64(n)) }
func ObserveCitationCoverage(c float64)       { citationCoverage.Observe(c) }
func SetActiveWorkers(n int)                  { activeWorkers.Set(float64(n)) }
func SetWorkerPoolSize(n int)                 { workerPoolSize.Set(float64(n)) }
func SetQueueDepth(n int)                     { queueDepth.Set(float64(n)) }
func RecordVerificationFailure()              { verificationFailures.Inc() }
func RecordTokenUsage(phase string, tokens int64) {
	tokenUsage.WithLabelValues(phase).Add(float64(tokens))
}
