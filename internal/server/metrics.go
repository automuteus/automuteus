package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type EventType int

// EventType is a coarse category of Discord activity. Mute/deafen changes are not here; they have their own
// metrics (see VoiceRoute) with outcomes and durations.
const (
	MessageCreateDelete EventType = iota
	MessageEdit
	RateLimited
)

var metricTypeStrings = [...]string{
	"message_create_delete",
	"message_edit",
	"rate_limited",
}

// VoiceRoute is the path that finally applied (or failed to apply) a mute/deafen for one user.
type VoiceRoute string

const (
	VoiceRoutePrimary VoiceRoute = "primary" // the bot's own token
	VoiceRouteWorker  VoiceRoute = "worker"  // a premium worker token
	VoiceRouteCapture VoiceRoute = "capture" // the capture client's own token
)

var voiceRoutes = [...]VoiceRoute{VoiceRoutePrimary, VoiceRouteWorker, VoiceRouteCapture}

// CaptureTaskResult is what happened to one mute/deafen task handed to a capture client.
type CaptureTaskResult string

const (
	CaptureTaskApplied   CaptureTaskResult = "applied"   // the capture client acknowledged the change
	CaptureTaskThrottled CaptureTaskResult = "throttled" // the bot withheld the task to protect the client's rate limit
	CaptureTaskUnacked   CaptureTaskResult = "unacked"   // no ack before the timeout; the client is blacklisted
	CaptureTaskError     CaptureTaskResult = "error"     // the task could not be published or subscribed to
)

var captureTaskResults = [...]CaptureTaskResult{CaptureTaskApplied, CaptureTaskThrottled, CaptureTaskUnacked, CaptureTaskError}

// EndReason is why a game was ended. It is a small fixed set; the details (who, which notice) belong in logs.
type EndReason string

const (
	EndReasonManual          EndReason = "manual"           // /end
	EndReasonReplaced        EndReason = "replaced"         // /new in a channel that already had a game
	EndReasonInactivity      EndReason = "inactivity"       // the capture stopped sending events
	EndReasonCaptureShutdown EndReason = "capture_shutdown" // the capture service announced a restart
	EndReasonCriticalNotice  EndReason = "critical_notice"  // operators ended every game
	EndReasonOther           EndReason = "other"
)

var endReasons = [...]EndReason{EndReasonManual, EndReasonReplaced, EndReasonInactivity, EndReasonCaptureShutdown, EndReasonCriticalNotice, EndReasonOther}

// CleanupStep is one of the things that must happen when a game ends early.
type CleanupStep string

const (
	CleanupUnmute      CleanupStep = "unmute"       // players were left muted or deafened
	CleanupRecordMatch CleanupStep = "record_match" // the match was not marked aborted in Postgres
	CleanupNotify      CleanupStep = "notify"       // the end-of-game message was not posted
)

var cleanupSteps = [...]CleanupStep{CleanupUnmute, CleanupRecordMatch, CleanupNotify}

// Metrics holds every Prometheus collector for one bot process, across all of its shards.
type Metrics struct {
	// operations are activity counters, not an exact count of HTTP requests or successful responses.
	operations *prometheus.CounterVec

	voiceChanges      *prometheus.CounterVec
	workerFailures    prometheus.Counter
	captureTasks      *prometheus.CounterVec
	muteBatchDuration prometheus.Histogram

	activeGames     prometheus.Gauge
	gamesStarted    prometheus.Counter
	gamesEnded      *prometheus.CounterVec
	cleanupFailures *prometheus.CounterVec
}

// DefaultMetrics is shared by the bot and token provider and is registered once per process.
var DefaultMetrics = NewMetrics(prometheus.DefaultRegisterer)

func NewMetrics(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_discord_operations_total",
			Help: "Coarse Discord message activity and observed rate limits in this process; not an exact HTTP request count.",
		}, []string{"type"}),
		voiceChanges: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_voice_changes_total",
			Help: "Final outcome of each requested mute/deafen change, by the route that last handled it.",
		}, []string{"route", "outcome"}),
		workerFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automuteus_worker_voice_failures_total",
			Help: "Mute/deafen attempts on a premium worker token that failed and fell back to another route.",
		}),
		captureTasks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_capture_mute_tasks_total",
			Help: "Mute/deafen tasks routed to capture clients, by result. Games without a capture client able to mute are not counted.",
		}, []string{"result"}),
		muteBatchDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "automuteus_mute_batch_duration_seconds",
			Help:    "Wall time to apply one batch of mute/deafen changes across all routes.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}),
		activeGames: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "automuteus_active_games",
			Help: "Games whose capture events this process is currently subscribed to.",
		}),
		gamesStarted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automuteus_games_started_total",
			Help: "Games started with /new in this process.",
		}),
		gamesEnded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_games_ended_total",
			Help: "Games ended in this process, by reason.",
		}, []string{"reason"}),
		cleanupFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_game_cleanup_failures_total",
			Help: "End-of-game cleanup steps that failed, by step.",
		}, []string{"step"}),
	}

	// Expose every label combination as zero so absent series read as "nothing happened", not "not scraped".
	for _, name := range metricTypeStrings {
		m.operations.WithLabelValues(name)
	}
	for _, route := range voiceRoutes {
		m.voiceChanges.WithLabelValues(string(route), "applied")
		m.voiceChanges.WithLabelValues(string(route), "failed")
	}
	for _, result := range captureTaskResults {
		m.captureTasks.WithLabelValues(string(result))
	}
	for _, reason := range endReasons {
		m.gamesEnded.WithLabelValues(string(reason))
	}
	for _, step := range cleanupSteps {
		m.cleanupFailures.WithLabelValues(string(step))
	}

	registry.MustRegister(m.operations, m.voiceChanges, m.workerFailures, m.captureTasks, m.muteBatchDuration,
		m.activeGames, m.gamesStarted, m.gamesEnded, m.cleanupFailures)
	return m
}

func (m *Metrics) RecordDiscordRequests(requestType EventType, num int64) {
	if num > 0 {
		m.operations.WithLabelValues(metricTypeStrings[requestType]).Add(float64(num))
	}
}

// RecordVoiceChange counts the final outcome for one user in a mute/deafen batch, after any fallbacks.
func (m *Metrics) RecordVoiceChange(route VoiceRoute, applied bool) {
	outcome := "failed"
	if applied {
		outcome = "applied"
	}
	m.voiceChanges.WithLabelValues(string(route), outcome).Inc()
}

// RecordWorkerFailure counts a worker-token attempt that failed; the user is retried on another route.
func (m *Metrics) RecordWorkerFailure() {
	m.workerFailures.Inc()
}

// RecordCaptureTask counts one task handed to (or withheld from) a capture client.
func (m *Metrics) RecordCaptureTask(result CaptureTaskResult) {
	m.captureTasks.WithLabelValues(string(result)).Inc()
}

// ObserveMuteBatch records how long one ModifyUsers call took end to end.
func (m *Metrics) ObserveMuteBatch(elapsed time.Duration) {
	m.muteBatchDuration.Observe(elapsed.Seconds())
}

// AddActiveGames adjusts the process-wide count when a shard starts or stops
// tracking a game. Each shard must report only changes to its tracked set.
func (m *Metrics) AddActiveGames(delta int) {
	m.activeGames.Add(float64(delta))
}

func (m *Metrics) RecordGameStarted() {
	m.gamesStarted.Inc()
}

// RecordGameEnded counts a game ending. Reasons outside the fixed set are folded into "other".
func (m *Metrics) RecordGameEnded(reason EndReason) {
	m.gamesEnded.WithLabelValues(string(reason.known())).Inc()
}

// RecordCleanupFailure counts an end-of-game step that did not complete.
func (m *Metrics) RecordCleanupFailure(step CleanupStep) {
	m.cleanupFailures.WithLabelValues(string(step)).Inc()
}

// known maps arbitrary reasons onto the fixed label set, so a new caller cannot grow the metric's cardinality.
func (r EndReason) known() EndReason {
	for _, known := range endReasons {
		if r == known {
			return r
		}
	}
	return EndReasonOther
}

func metricsHandler(gatherer prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	return mux
}

func PrometheusMetricsServer(port string) error {
	return http.ListenAndServe(":"+port, metricsHandler(prometheus.DefaultGatherer))
}
