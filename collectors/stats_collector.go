package collectors

import (
	"github.com/DRuggeri/bind_query_exporter/util"
	"github.com/prometheus/client_golang/prometheus"
)

type StatCollector struct {
	namespace     string
	statMetric    prometheus.Counter
	typesMetric   prometheus.CounterVec
	clientsMetric prometheus.CounterVec
}

func NewStatsCollector(namespace string, sender *chan string, matcher *util.LogMatcher, captureClient bool) *StatCollector {
	config := tailConfig{
		matcher:       matcher,
		captureClient: captureClient,
	}

	statMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "total",
			Help:      "Total queries recieved",
		},
	)

	typesMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "total_by_type",
			Help:      "Total queries recieved by type of query",
		},
		[]string{"type"},
	)

	clientsMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "by_client_and_type",
			Help:      "Total queries recieved by type of query by client",
		},
		[]string{"type", "client"},
	)

	/* Spin off a thread that will gather our data on every read from the file */
	go func(sender *chan string, clientsMetric *prometheus.CounterVec, statMetric prometheus.Counter, typesMetric *prometheus.CounterVec, matcher *util.LogMatcher, config *tailConfig) {
		for line := range *sender {
			info := matcher.ExtractInfo(line)
			if info.Matched {
				statMetric.Add(1)
				typesMetric.WithLabelValues(info.QueryType).Add(1)
				if config.captureClient {
					clientsMetric.WithLabelValues(info.QueryType, info.QueryClient).Add(1)
				}
			}
		}
	}(sender, clientsMetric, statMetric, typesMetric, matcher, &config)

	return &StatCollector{
		namespace:     namespace,
		statMetric:    statMetric,
		typesMetric:   *typesMetric,
		clientsMetric: *clientsMetric,
	}
}

func (c *StatCollector) Collect(ch chan<- prometheus.Metric) {
	c.statMetric.Collect(ch)
	c.typesMetric.Collect(ch)
	c.clientsMetric.Collect(ch)
}

func (c *StatCollector) Describe(ch chan<- *prometheus.Desc) {
	c.statMetric.Describe(ch)
	c.typesMetric.Describe(ch)
	c.clientsMetric.Describe(ch)
}
