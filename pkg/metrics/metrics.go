package metrics

import (
	"context"
	"errors"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strconv"
)

type EventType int

const (
	MuteDeafenOfficial EventType = iota
	MessageCreateDelete
	MessageEdit
	ReactionAdd
	MuteDeafenCapture
	MuteDeafenWorker
	InvalidRequest
	OfficialRequest //must be the last metric
)

var MetricTypeStrings = []string{
	"mute_deafen_official",
	"message_create_delete",
	"message_edit",
	"reaction_add_remove",
	"mute_deafen_capture",
	"mute_deafen_worker",
	"invalid_request",
	"official_request", //must be the last request
}

type Collector struct {
	counterDesc *prometheus.Desc
	client      *redis.Client
	commit      string
	nodeID      string
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	official := int64(0)
	for i, str := range MetricTypeStrings {
		if i != int(OfficialRequest) {
			v, err := c.client.Get(context.Background(), rediskey.RequestsByType(str)).Result()
			if !errors.Is(err, redis.Nil) && err != nil {
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
				if i != int(MuteDeafenCapture) && i != int(MuteDeafenWorker) {
					official += num
				}
			}
		} else {
			ch <- prometheus.MustNewConstMetric(
				c.counterDesc,
				prometheus.CounterValue,
				float64(official),
				c.nodeID,
				str,
			)
		}
	}
}

func RecordDiscordRequests(client *redis.Client, requestType EventType, num int64) {
	for i := int64(0); i < num; i++ {
		typeStr := MetricTypeStrings[requestType]
		client.Incr(context.Background(), rediskey.RequestsByType(typeStr))
	}
}

func NewCollector(client *redis.Client, nodeID string) *Collector {
	return &Collector{
		counterDesc: prometheus.NewDesc("discord_requests_by_node_and_type", "Number of discord requests made, differentiated by node/type", []string{"nodeID", "type"}, nil),
		client:      client,
		nodeID:      nodeID,
	}
}

func PrometheusMetricsServer(client *redis.Client, nodeID, port string) error {
	prometheus.MustRegister(NewCollector(client, nodeID))

	http.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(":"+port, nil)
}
