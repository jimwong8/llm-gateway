package longcontext

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Task lifecycle metrics
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

	// Phase duration histogram
	phaseDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "longcontext_phase_duration_seconds",
		Help: "Duration of each task phase in seconds",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
	}, []string{"phase"})

	// Chunk metrics
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
		Help: "Duration of chunk map phase in seconds",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
	})

	// Evidence metrics
	evidenceGenerated = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "longcontext_evidence_count",
		Help: "Number of evidence items generated per task",
		Buckets: []float64{0, 1, 5, 10, 20, 50, 100},
	})

	citationCoverage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "longcontext_citation_coverage",
		Help: "Citation coverage ratio per task",
		Buckets: []float64{0, 0.1, 0.2, 0.3, 0.5, 0.8, 1.0},
	})

	// Worker metrics
	activeWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_active_workers",
		Help: "Number of currently active long-context workers",
	})

	workerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_worker_pool_size",
		Help: "Configured long-context worker pool size",
	})

	// Queue metrics
	queueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "longcontext_queue_depth",
		Help: "Number of tasks waiting in queue (status=queued)",
	})

	// Verification metrics
	verificationFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "longcontext_verification_failures_total",
		Help: "Total number of verification failures",
	})

	// Cost metrics
	tokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "longcontext_tokens_used_total",
		Help: "Total tokens used by long-context tasks",
	}, []string{"phase"})
)

// RecordTaskCreated increments the tasks_created counter.
func RecordTaskCreated() { tasksCreated.Inc() }

// RecordTaskSucceeded increments the tasks_succeeded counter.
func RecordTaskSucceeded() { tasksSucceeded.Inc() }

// RecordTaskFailed increments the tasks_failed counter with a reason label.
func RecordTaskFailed(reason string) {
	tasksFailed.WithLabelValues(reason).Inc()
}

// ObservePhaseDuration records the duration of a task phase.
func ObservePhaseDuration(phase string, d time.Duration) {
	phaseDuration.WithLabelValues(phase).Observe(d.Seconds())
}

// RecordChunkProcessed increments the chunks_processed counter.
func RecordChunkProcessed() { chunksProcessed.Inc() }

// RecordChunkFailed increments the chunks_failed counter.
func RecordChunkFailed() { chunksFailed.Inc() }

// ObserveChunkMapDuration records chunk map duration.
func ObserveChunkMapDuration(d time.Duration) {
	chunkMapDuration.Observe(d.Seconds())
}

// ObserveEvidenceCount records the number of evidence items.
func ObserveEvidenceCount(n int) { evidenceGenerated.Observe(float64(n)) }

// ObserveCitationCoverage records citation coverage.
func ObserveCitationCoverage(coverage float64) { citationCoverage.Observe(coverage) }

// SetActiveWorkers sets the active workers gauge.
func SetActiveWorkers(n int) { activeWorkers.Set(float64(n)) }

// SetWorkerPoolSize sets the worker pool size gauge.
func SetWorkerPoolSize(n int) { workerPoolSize.Set(float64(n)) }

// SetQueueDepth sets the queue depth gauge.
func SetQueueDepth(n int) { queueDepth.Set(float64(n)) }

// RecordVerificationFailure increments the verification_failures counter.
func RecordVerificationFailure() { verificationFailures.Inc() }

// RecordTokenUsage records token usage for a phase.
func RecordTokenUsage(phase string, tokens int64) {
	tokenUsage.WithLabelValues(phase).Add(float64(tokens))
}
