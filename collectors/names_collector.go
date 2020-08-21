package collectors

import (
	"bufio"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"net"
	"os"
	"regexp"
	"time"
)

type NamesCollector struct {
	namespace   string
	namesMetric *prometheus.CounterVec
	totalMetric prometheus.Counter
	stats       map[string]float64

	scrapesTotalMetric              prometheus.Counter
	scrapeErrorsTotalMetric         prometheus.Counter
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

type tailConfig struct {
	pattern       string
	include       map[string]bool
	exclude       map[string]bool
	captureClient bool
	reverseLookup bool
}

func NewNamesCollector(namespace string, sender *chan string, pattern string, includeFile string, excludeFile string, captureClient bool, reverseLookup bool) (*NamesCollector, error) {
	stats := make(map[string]float64)

	config := tailConfig{
		pattern:       pattern,
		include:       make(map[string]bool),
		exclude:       make(map[string]bool),
		captureClient: captureClient,
		reverseLookup: reverseLookup,
	}

	if "" != includeFile {
		log.Infoln("Will only export names that ARE in the file ", includeFile)
		err := makeList(includeFile, &config.include)
		if err != nil {
			log.Errorln("Failed to use include file: ", includeFile, err)
			return nil, err
		}
	}
	if "" != excludeFile {
		log.Infoln("Will only export names that ARE NOT the file ", excludeFile)
		err := makeList(excludeFile, &config.exclude)
		if err != nil {
			log.Errorln("Failed to use exclude file: ", excludeFile, err)
			return nil, err
		}
	}

	var namesMetric *prometheus.CounterVec
	if captureClient {
		namesMetric = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "names",
				Name:      "all",
				Help:      "Queries per DNS name per client",
			},
			[]string{"name", "client"},
		)
	} else {
		namesMetric = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "names",
				Name:      "all",
				Help:      "Queries per DNS name",
			},
			[]string{"name"},
		)
	}

	totalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "total",
			Help:      "Sum of all queries matched. If no include/exclude filter is present, this will match bind_query_stats_total in the stats collector.  It is initialized to 0 to support increment() detection.",
		},
	)
	totalMetric.Add(0)

	/* Spin off a thread that will gather our data on every read from the file */
	go func(sender *chan string, namesMetric *prometheus.CounterVec, totalMetric prometheus.Counter, config *tailConfig) {
		re := regexp.MustCompile(config.pattern)

		for line := range *sender {
			log.Debugln(line)
			match := re.FindStringSubmatch(line)
			if len(match) > 0 {
				increment := false
				client := match[1]
				name := match[2]

				if len(config.include) > 0 {
					if _, ok := config.include[name]; ok {
						increment = true
					}
				} else if len(config.exclude) > 0 {
					if _, ok := config.exclude[name]; !ok {
						increment = true
					}
				} else {
					//(*stats)[match[2]]++
					increment = true
				}

				if increment {
					totalMetric.Add(1)
					if config.reverseLookup {
						if names, dnsErr := net.LookupAddr(client); dnsErr == nil && len(names) > 0 {
							client = names[0]
						}
					}
					if config.captureClient {
						namesMetric.WithLabelValues(name, client).Add(1)
					} else {
						namesMetric.WithLabelValues(name).Add(1)
					}
				}
			}
		}
	}(sender, namesMetric, totalMetric, &config)

	scrapesTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "scrapes_total",
			Help:      "Total number of scrapes for BIND names stats.",
		},
	)

	scrapeErrorsTotalMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "scrape_errors_total",
			Help:      "Total number of scrapes errors for BIND names stats.",
		},
	)

	lastScrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of BIND names stats resulted in an error (1 for error, 0 for success).",
		},
	)

	lastScrapeTimestampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "last_scrape_timestamp",
			Help:      "Number of seconds since 1970 since last scrape of BIND names metrics.",
		},
	)

	lastScrapeDurationSecondsMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "names",
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of BIND names stats.",
		},
	)

	return &NamesCollector{
		stats:       stats,
		namespace:   namespace,
		namesMetric: namesMetric,
		totalMetric: totalMetric,

		scrapesTotalMetric:              scrapesTotalMetric,
		scrapeErrorsTotalMetric:         scrapeErrorsTotalMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}, nil
}

func makeList(fileName string, result *map[string]bool) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		log.Debugln("  ", scanner.Text())
		(*result)[scanner.Text()] = true
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (c *NamesCollector) Collect(ch chan<- prometheus.Metric) {
	var begun = time.Now()

	errorMetric := float64(0)
	c.totalMetric.Collect(ch)
	c.namesMetric.Collect(ch)

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

func (c *NamesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.namesMetric.Describe(ch)
	c.totalMetric.Describe(ch)
	c.scrapesTotalMetric.Describe(ch)
	c.scrapeErrorsTotalMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}
