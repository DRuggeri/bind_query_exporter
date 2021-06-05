package collectors

import (
	"bufio"
	"github.com/DRuggeri/bind_query_exporter/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"os"
)

type NamesCollector struct {
	namespace   string
	namesMetric *prometheus.CounterVec
	totalMetric prometheus.Counter
}

func NewNamesCollector(namespace string, sender *chan string, matcher *util.LogMatcher, includeFile string, excludeFile string, captureClient bool) (*NamesCollector, error) {
	config := tailConfig{
		matcher:       matcher,
		captureClient: captureClient,
	}

	if "" != includeFile {
		log.Infoln("Will only export names that ARE in the file ", includeFile)
		matcher.Include = make(map[string]bool)
		err := makeList(includeFile, &matcher.Include)
		if err != nil {
			log.Errorln("Failed to use include file: ", includeFile, err)
			return nil, err
		}
	}
	if "" != excludeFile {
		log.Infoln("Will only export names that ARE NOT the file ", excludeFile)
		matcher.Exclude = make(map[string]bool)
		err := makeList(excludeFile, &matcher.Exclude)
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
		for line := range *sender {
			info := matcher.ExtractInfo(line)
			if info.Matched {
				totalMetric.Add(1)
				if config.captureClient {
					namesMetric.WithLabelValues(info.QueryName, info.QueryClient).Add(1)
				} else {
					namesMetric.WithLabelValues(info.QueryName).Add(1)
				}
			}
		}
	}(sender, namesMetric, totalMetric, &config)

	return &NamesCollector{
		namespace:   namespace,
		namesMetric: namesMetric,
		totalMetric: totalMetric,
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
	c.totalMetric.Collect(ch)
	c.namesMetric.Collect(ch)
}

func (c *NamesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.namesMetric.Describe(ch)
	c.totalMetric.Describe(ch)
}
