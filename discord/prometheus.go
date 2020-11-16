package discord

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type MetricsObserver struct {
	NodeID           string
	MetricsCollector *MetricsCollector
}

// Descriptors used by the RedisObserverCollector below.
var (
	discord1minDesc = prometheus.NewDesc(
		"discord_requests_per_1m",
		"Number of Discord API requests in the past 1 minute",
		[]string{"nodeID", "type"}, nil,
	)
	discord10minDesc = prometheus.NewDesc(
		"discord_requests_per_10m",
		"Number of Discord API requests in the past 10 minutes",
		[]string{"nodeID", "type"}, nil,
	)
)

func (cc MetricsObserverCollector) Collect(ch chan<- prometheus.Metric) {
	//get rid of old entries
	cc.MetricsObserver.MetricsCollector.prune()

	ch <- prometheus.MustNewConstMetric(
		discord1minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute, Generic)),
		cc.MetricsObserver.NodeID,
		"all",
	)

	ch <- prometheus.MustNewConstMetric(
		discord10minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute*10, Generic)),
		cc.MetricsObserver.NodeID,
		"all",
	)

	ch <- prometheus.MustNewConstMetric(
		discord1minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute, MuteDeafen)),
		cc.MetricsObserver.NodeID,
		"mute/deafen",
	)

	ch <- prometheus.MustNewConstMetric(
		discord10minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute*10, MuteDeafen)),
		cc.MetricsObserver.NodeID,
		"mute/deafen",
	)

	ch <- prometheus.MustNewConstMetric(
		discord1minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute, MessageCreateDelete)),
		cc.MetricsObserver.NodeID,
		"create/delete",
	)

	ch <- prometheus.MustNewConstMetric(
		discord10minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute*10, MessageCreateDelete)),
		cc.MetricsObserver.NodeID,
		"create/delete",
	)

	ch <- prometheus.MustNewConstMetric(
		discord1minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute, MessageEdit)),
		cc.MetricsObserver.NodeID,
		"edit",
	)

	ch <- prometheus.MustNewConstMetric(
		discord10minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute*10, MessageEdit)),
		cc.MetricsObserver.NodeID,
		"edit",
	)

	ch <- prometheus.MustNewConstMetric(
		discord1minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute, ReactionAdd)),
		cc.MetricsObserver.NodeID,
		"reaction",
	)

	ch <- prometheus.MustNewConstMetric(
		discord10minDesc,
		prometheus.GaugeValue,
		float64(cc.MetricsObserver.MetricsCollector.TotalRequestCountInTimeFiltered(time.Minute*10, ReactionAdd)),
		cc.MetricsObserver.NodeID,
		"reaction",
	)
}

func (bot *Bot) NewDiscordAPIRequestObserver(nodeID string, reg prometheus.Registerer) *MetricsObserver {
	c := &MetricsObserver{
		NodeID:           nodeID,
		MetricsCollector: bot.MetricsCollector,
	}
	cc := MetricsObserverCollector{MetricsObserver: c}
	prometheus.WrapRegistererWith(nil, reg).MustRegister(cc)
	return c
}

func (bot *Bot) PrometheusMetricsServer(nodeID, port string) error {

	reg := prometheus.NewPedanticRegistry()

	bot.NewDiscordAPIRequestObserver(nodeID, reg)

	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return http.ListenAndServe(":"+port, nil)
}
