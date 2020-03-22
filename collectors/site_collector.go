package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"regexp"
	"time"
)

type SitesCollector struct {
	namespace   string
	sitesMetric prometheus.CounterVec
	stats       map[string]float64

	scrapesTotalMetric              prometheus.Counter
	scrapeErrorsTotalMetric         prometheus.Counter
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

func NewSitesCollector(namespace string, sender *chan string) *SitesCollector {
	stats := make(map[string]float64)

	/* Spin off a thread that will gather our data on every read from the file */
	go func(sender *chan string, stats *map[string]float64) {
		//22-Mar-2020 14:54:27.568 queries: info: client 192.168.0.1#63519 (www.google.com): query: www.google.com IN A + (192.168.0.100)
		re := regexp.MustCompile(`query: ([^\s]+)`)

		for line := range *sender {
			log.Debugln(line)
			match := re.FindStringSubmatch(line)
			if len(match) > 0 {
				(*stats)[match[1]]++
			}
		}
	}(sender, &stats)

	sitesMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "sites",
			Name:      "number",
			Help:      "Queries per DNS name",
		},
		[]string{"domain"},
	)

	scrapesTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "sites_scrapes",
			Name:      "total",
			Help:      "Total number of scrapes for BIND sites stats.",
		},
	)

	scrapeErrorsTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "sites_scrape_errors",
			Name:      "total",
			Help:      "Total number of scrapes errors for BIND sites stats.",
		},
	)

	lastScrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "",
			Name:      "last_sites_scrape_error",
			Help:      "Whether the last scrape of BIND sites stats resulted in an error (1 for error, 0 for success).",
		},
	)

	lastScrapeTimestampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "",
			Name:      "last_sites_scrape_timestamp",
			Help:      "Number of seconds since 1970 since last scrape of BIND sites metrics.",
		},
	)

	lastScrapeDurationSecondsMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "",
			Name:      "last_sites_scrape_duration_seconds",
			Help:      "Duration of the last scrape of BIND sites stats.",
		},
	)

	return &SitesCollector{
		stats:       stats,
		namespace:   namespace,
		sitesMetric: *sitesMetric,

		scrapesTotalMetric:              scrapesTotalMetric,
		scrapeErrorsTotalMetric:         scrapeErrorsTotalMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}
}

func (c *SitesCollector) Collect(ch chan<- prometheus.Metric) {
	var begun = time.Now()

	errorMetric := float64(0)
	for k, v := range c.stats {
		c.sitesMetric.WithLabelValues(k).Add(v)
		delete(c.stats, k)
	}
	c.sitesMetric.Collect(ch)

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

func (c *SitesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.sitesMetric.Describe(ch)
	c.scrapesTotalMetric.Describe(ch)
	c.scrapeErrorsTotalMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}
