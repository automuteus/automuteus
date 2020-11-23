package metrics

import (
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

const MAX_TTL_MINUTES = 10

type MetricsEventType int

const (
	Generic MetricsEventType = iota
	MuteDeafen
	MessageCreateDelete
	MessageEdit
	ReactionAdd
)

type MetricsCollector struct {
	data map[int64]MetricsEventType
	lock sync.RWMutex
}

// Describe is implemented with DescribeByCollect. That's possible because the
// Collect method will always return the same two metrics with the same two
// descriptors.
func (cc MetricsObserverCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(cc, ch)
}

// RedisObserverCollector implements the Collector interface.
type MetricsObserverCollector struct {
	MetricsObserver *MetricsObserver
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		data: make(map[int64]MetricsEventType),
		lock: sync.RWMutex{},
	}
}

func (mc *MetricsCollector) RecordDiscordRequests(client *redis.Client, requestType MetricsEventType, num int64) {
	t := time.Now().UnixNano()

	mc.lock.Lock()
	for i := int64(0); i < num; i++ {
		mc.data[t+i] = requestType
	}

	mc.lock.Unlock()
	go incrementDiscordRequests(client, num)
}

func (mc *MetricsCollector) TotalRequestCountInTimeFiltered(timeBack time.Duration, filter MetricsEventType) int64 {
	cutoff := time.Now().Add(-timeBack).UnixNano()

	count := int64(0)
	mc.lock.RLock()
	for i, v := range mc.data {
		if i > cutoff {
			if filter == Generic || filter == v {
				count++
			}
		} else {
			break
		}
	}
	mc.lock.RUnlock()
	return count
}

func (mc *MetricsCollector) prune() {
	oldest := time.Now().Add(-MAX_TTL_MINUTES * time.Minute).UnixNano()
	mc.lock.Lock()
	for t := range mc.data {
		if t < oldest {
			delete(mc.data, t)
		}
	}
	mc.lock.Unlock()
}
