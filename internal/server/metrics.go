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

// AdoptSource is how a process came to subscribe to a game another process created.
type AdoptSource string

const (
	AdoptAnnounce    AdoptSource = "announce"     // the creating or draining process announced it
	AdoptDiscovery   AdoptSource = "discovery"    // the periodic scan of recently active games found it
	AdoptGuildCreate AdoptSource = "guild_create" // the shard reconnected and resubscribed to the guild's games
)

var adoptSources = [...]AdoptSource{AdoptAnnounce, AdoptDiscovery, AdoptGuildCreate}

// Metrics holds every Prometheus collector for one bot process, across all of its shards.
type Metrics struct {
	workerCleanup        *prometheus.CounterVec
	workerCleanupPending prometheus.Gauge
	workerCleanupOldest  prometheus.Gauge
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

	// Consumer-lease and handover activity between processes sharing a shard. See bot/lease.go.
	leaseLost       prometheus.Counter
	leaseWaits      prometheus.Counter
	gamesAdopted    *prometheus.CounterVec
	gamesHandedOver prometheus.Counter
}

// DefaultMetrics is shared by the bot and token provider and is registered once per process.
var DefaultMetrics = NewMetrics(prometheus.DefaultRegisterer)

func NewMetrics(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		workerCleanup: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_worker_cleanup_total",
			Help: "Background worker cleanup checks, departures, deferrals, and failures.",
		}, []string{"result"}),
		workerCleanupPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "automuteus_worker_cleanup_pending_guilds",
			Help: "Latest observed fleet-wide number of guilds queued for worker departures.",
		}),
		workerCleanupOldest: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "automuteus_worker_cleanup_oldest_check_seconds",
			Help: "Latest observed age of the oldest scheduled guild check attempt across the fleet.",
		}),
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
		leaseLost: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automuteus_consumer_lease_lost_total",
			Help: "Times this process found a game's consumer lease held elsewhere while it believed it held it: a renewal failed or a burst stalled past the lease TTL. Should stay at zero.",
		}),
		leaseWaits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automuteus_consumer_lease_waits_total",
			Help: "End-of-game requests that had to wait for another process to finish a burst of capture events before cleaning up.",
		}),
		gamesAdopted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "automuteus_games_adopted_total",
			Help: "Games created by another process that this process subscribed to as a standby consumer, by how it learned of them.",
		}, []string{"source"}),
		gamesHandedOver: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "automuteus_games_handed_over_total",
			Help: "Games whose consumer lease this process released mid-burst while draining, leaving queued events for a standby.",
		}),
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
	for _, source := range adoptSources {
		m.gamesAdopted.WithLabelValues(string(source))
	}

	registry.MustRegister(m.operations, m.voiceChanges, m.workerFailures, m.captureTasks, m.muteBatchDuration,
		m.activeGames, m.gamesStarted, m.gamesEnded, m.cleanupFailures,
		m.leaseLost, m.leaseWaits, m.gamesAdopted, m.gamesHandedOver)
	for _, result := range []string{"checked", "left", "deferred", "failed", "rate_limited"} {
		m.workerCleanup.WithLabelValues(result)
	}
	registry.MustRegister(m.workerCleanup, m.workerCleanupPending, m.workerCleanupOldest)
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

func (m *Metrics) RecordWorkerCleanup(result string) { m.workerCleanup.WithLabelValues(result).Inc() }

func (m *Metrics) SetWorkerCleanupStatus(pending, oldestSeconds float64) {
	m.workerCleanupPending.Set(pending)
	m.workerCleanupOldest.Set(oldestSeconds)
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

// RecordLeaseLost counts a consumer lease this process believed it held turning out to be held elsewhere.
func (m *Metrics) RecordLeaseLost() {
	m.leaseLost.Inc()
}

// RecordLeaseWait counts an end-of-game request that found the consumer lease taken and had to wait for it.
func (m *Metrics) RecordLeaseWait() {
	m.leaseWaits.Inc()
}

// RecordGameAdopted counts a subscription to a game another process created. Unknown sources fold into "announce"
// so a new caller cannot grow the metric's cardinality.
func (m *Metrics) RecordGameAdopted(source AdoptSource) {
	m.gamesAdopted.WithLabelValues(string(source.known())).Inc()
}

// RecordGameHandedOver counts a game whose lease this process released mid-burst while draining.
func (m *Metrics) RecordGameHandedOver() {
	m.gamesHandedOver.Inc()
}

func (s AdoptSource) known() AdoptSource {
	for _, known := range adoptSources {
		if s == known {
			return s
		}
	}
	return AdoptAnnounce
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
