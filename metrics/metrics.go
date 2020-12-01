package metrics

import (
	"context"
	redis_common "github.com/denverquane/amongusdiscord/common"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strconv"
)

type MetricsEventType int

const (
	Generic MetricsEventType = iota
	MuteDeafenOfficial
	MessageCreateDelete
	MessageEdit
	ReactionAdd
	MuteDeafenCapture
	MuteDeafenWorker
)

var MetricTypeStrings = []string{
	"Generic",
	"mute_deafen_official",
	"message_create_delete",
	"message_edit",
	"reaction_add_remove",
	"mute_deafen_capture",
	"mute_deafen_worker",
}

type MetricsCollector struct {
	counterDesc *prometheus.Desc
	client      *redis.Client
	commit      string
	nodeID      string
}

func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	for i, str := range MetricTypeStrings {
		if i != 0 {
			v, err := c.client.Get(context.Background(), discordRequestsKeyByCommitAndType(c.commit, str)).Result()
			if err != redis.Nil && err != nil {
				log.Println(err)
				continue
			} else {
				num := int64(0)
				if v != "" {
					num, err = strconv.ParseInt(v, 10, 64)
					if err != nil {
						log.Println(err)
						num = 0
					}
				}

				ch <- prometheus.MustNewConstMetric(
					c.counterDesc,
					prometheus.CounterValue,
					float64(num),
					c.nodeID,
					str,
				)
			}
		}
	}

}

func RecordDiscordRequests(client *redis.Client, requestType MetricsEventType, num int64) {
	go incrementDiscordRequests(client, requestType, num)
}

func NewCollector(client *redis.Client, commit, nodeID string) *MetricsCollector {
	return &MetricsCollector{
		counterDesc: prometheus.NewDesc("discord_requests_by_node_and_type", "Number of discord requests made, differentiated by node/type", []string{"nodeID", "type"}, nil),
		client:      client,
		commit:      commit,
		nodeID:      nodeID,
	}
}

func PrometheusMetricsServer(client *redis.Client, nodeID, port string) error {
	_, comm := redis_common.GetVersionAndCommit(client)

	prometheus.MustRegister(NewCollector(client, comm, nodeID))

	http.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(":"+port, nil)
}
