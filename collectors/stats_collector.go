package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"regexp"
	"time"
)

type StatCollector struct {
	namespace   string
	statMetric  prometheus.Counter
	typesMetric prometheus.CounterVec
	stats       map[string]float64

	scrapesTotalMetric              prometheus.Counter
	scrapeErrorsTotalMetric         prometheus.Counter
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

func NewStatsCollector(namespace string, sender *chan string) *StatCollector {
	stats := make(map[string]float64)

	/* Spin off a thread that will gather our data on every read from the file */
	go func(sender *chan string, stats *map[string]float64) {
		//22-Mar-2020 14:54:27.568 queries: info: client 192.168.0.1#63519 (www.google.com): query: www.google.com IN A + (192.168.0.100)
		re := regexp.MustCompile(`query: [^\s]+ IN ([^\s]+) ([+-])([\s]*)`)

		for line := range *sender {
			log.Debugln(line)
			match := re.FindStringSubmatch(line)
			if len(match) > 0 {
				(*stats)["total"]++
				(*stats)[match[1]]++
			}
		}
	}(sender, &stats)

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

	scrapesTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "scrapes_total",
			Help:      "Total number of scrapes for BIND query stats.",
		},
	)

	scrapeErrorsTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "scrape_errors_total",
			Help:      "Total number of scrapes errors for BIND query stats.",
		},
	)

	lastScrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of BIND query stats resulted in an error (1 for error, 0 for success).",
		},
	)

	lastScrapeTimestampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "last_scrape_timestamp",
			Help:      "Number of seconds since 1970 since last scrape of BIND qyery stat metrics.",
		},
	)

	lastScrapeDurationSecondsMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "stats",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of BIND query stats.",
		},
	)

	return &StatCollector{
		stats:       stats,
		namespace:   namespace,
		statMetric:  statMetric,
		typesMetric: *typesMetric,

		scrapesTotalMetric:              scrapesTotalMetric,
		scrapeErrorsTotalMetric:         scrapeErrorsTotalMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}
}

func (c *StatCollector) Collect(ch chan<- prometheus.Metric) {
	var begun = time.Now()

	errorMetric := float64(0)
	for k, v := range c.stats {
		if k == "total" {
			c.statMetric.Add(v)
		} else {
			c.typesMetric.WithLabelValues(k).Add(v)
		}
		delete(c.stats, k)
	}
	c.statMetric.Collect(ch)
	c.typesMetric.Collect(ch)

	c.scrapeErrorsTotalMetric.Collect(ch)

	c.scrapesTotalMetric.Inc()
	c.scrapesTotalMetric.Collect(ch)

	c.lastScrapeErrorMetric.Set(errorMetric)
	c.lastScrapeErrorMetric.Collect(ch)

	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))
	c.lastScrapeTimestampMetric.Collect(ch)

	c.lastScrapeDurationSecondsMetric.Set(time.Since(begun).Seconds())
	c.lastScrapeDurationSecondsMetric.Collect(ch)
}

func (c *StatCollector) Describe(ch chan<- *prometheus.Desc) {
	c.statMetric.Describe(ch)
	c.typesMetric.Describe(ch)
	c.scrapesTotalMetric.Describe(ch)
	c.scrapeErrorsTotalMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}
