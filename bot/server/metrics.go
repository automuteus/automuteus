package server

import (
	"errors"
	"github.com/automuteus/automuteus/v8/pkg/redis"
	redisv8 "github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strconv"
)

type Collector struct {
	counterDesc *prometheus.Desc
	driver      redis.Driver
	commit      string
	nodeID      string
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	official := int64(0)
	for i, str := range redis.MetricTypeStrings {
		if i != int(redis.OfficialRequest) {
			v, err := c.driver.GetRequestsByType(str)
			if !errors.Is(err, redisv8.Nil) && err != nil {
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
				if i != int(redis.MuteDeafenCapture) && i != int(redis.MuteDeafenWorker) {
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

func NewCollector(driver redis.Driver, nodeID string) *Collector {
	return &Collector{
		counterDesc: prometheus.NewDesc("discord_requests_by_node_and_type", "Number of discord requests made, differentiated by node/type", []string{"nodeID", "type"}, nil),
		driver:      driver,
		nodeID:      nodeID,
	}
}

func PrometheusMetricsServer(driver redis.Driver, nodeID, port string) error {
	prometheus.MustRegister(NewCollector(driver, nodeID))

	http.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(":"+port, nil)
}
