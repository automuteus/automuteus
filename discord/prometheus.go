package discord

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type RedisObserver struct {
	NodeID         string
	RedisInterface *RedisInterface
}

func (c *RedisObserver) ReallyExpensiveAssessmentOfTheSystemState() (Min1Counts, Min10Counts int) {
	Min1Counts = c.RedisInterface.GetDiscordRequestsInLastMinutesByNodeID(1, c.NodeID)
	Min10Counts = c.RedisInterface.GetDiscordRequestsInLastMinutesByNodeID(10, c.NodeID)
	return
}

// RedisObserverCollector implements the Collector interface.
type RedisObserverCollector struct {
	RedisObserver *RedisObserver
}

// Descriptors used by the RedisObserverCollector below.
var (
	discordRequests1minDesc = prometheus.NewDesc(
		"discordrequests_per_1m",
		"Number of Discord API requests in the past 1 minute by K8s Node ID",
		[]string{"nodeID"}, nil,
	)
	discordRequests10minDesc = prometheus.NewDesc(
		"discordrequests_per_10m",
		"Number of Discord API requests in the past 10 minutes by K8s Node ID",
		[]string{"nodeID"}, nil,
	)
)

// Describe is implemented with DescribeByCollect. That's possible because the
// Collect method will always return the same two metrics with the same two
// descriptors.
func (cc RedisObserverCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(cc, ch)
}

func (cc RedisObserverCollector) Collect(ch chan<- prometheus.Metric) {
	discReqs1Min, discReqs10Mins := cc.RedisObserver.ReallyExpensiveAssessmentOfTheSystemState()
	ch <- prometheus.MustNewConstMetric(
		discordRequests1minDesc,
		prometheus.GaugeValue,
		float64(discReqs1Min),
		cc.RedisObserver.NodeID,
	)

	ch <- prometheus.MustNewConstMetric(
		discordRequests10minDesc,
		prometheus.GaugeValue,
		float64(discReqs10Mins),
		cc.RedisObserver.NodeID,
	)
}

func (bot *Bot) NewDiscordAPIRequestObserver(nodeID string, reg prometheus.Registerer) *RedisObserver {
	c := &RedisObserver{
		NodeID:         nodeID,
		RedisInterface: bot.RedisInterface,
	}
	cc := RedisObserverCollector{RedisObserver: c}
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
