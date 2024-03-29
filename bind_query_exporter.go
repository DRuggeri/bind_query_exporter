package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/DRuggeri/bind_query_exporter/collectors"
	"github.com/DRuggeri/bind_query_exporter/filters"
	"github.com/DRuggeri/bind_query_exporter/util"
)

var Version = "testing"

var (
	bindQueryLogFile = kingpin.Flag(
		"log", "Path of the BIND query log to watch. Defaults to '/var/log/bind/queries.log' ($BIND_QUERY_EXPORTER_LOG)",
	).Envar("BIND_QUERY_EXPORTER_LOG").Default("/var/log/bind/queries.log").String()

	bindQueryPattern = kingpin.Flag(
		"pattern", "The regular expression pattern with three capturing matches for the client IP, the queried name, and the query type ($BIND_QUERY_EXPORTER_PATTERN)",
	).Envar("BIND_QUERY_EXPORTER_LOG").Default(util.LogMatcherDefaultPattern).String()

	bindQueryIncludeFile = kingpin.Flag(
		"names.include.file", "Path to a file of DNS names that this exporter WILL export when the Names filter is enabled. One DNS name per line will be read. ($BIND_QUERY_EXPORTER_NAMES_INCLUDE_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_INCLUDE_FILE").Default("").String()

	bindQueryExcludeFile = kingpin.Flag(
		"names.exclude.file", "Path to a file of DNS names that this exporter WILL NOT export when the Names filter is enabled. One DNS name per line will be read. ($BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE").Default("").String()

	bindQueryExcludeClientsFile = kingpin.Flag(
		"names.exclude-clients.file", "Path to a file of reverse names or IP addresses that this exporter will ignore. One entry per line will be read. ($BIND_QUERY_EXPORTER_NAMES_EXCLUDE_CLIENTS_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_EXCLUDE_CLIENTS_FILE").Default("").String()

	bindQueryIncludeClientsFile = kingpin.Flag(
		"names.include-clients.file", "Path to a file of reverse names or IP addresses that this exporter will capture. All others will be ignored. One entry per line will be read. ($BIND_QUERY_EXPORTER_NAMES_INCLUDE_CLIENTS_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_INCLUDE_CLIENTS_FILE").Default("").String()

	bindQueryNamesCaptureClient = kingpin.Flag(
		"names.capture-client", "Enable capturing the client making the client IP or name as part of the vector. WARNING: This will can lead to lots of metrics in your Prometheus database! ($BIND_QUERY_EXPORTER_NAMES_CAPTURE_CLIENT)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_CAPTURE_CLIENT").Default("false").Bool()

	bindQueryNamesReverseLookup = kingpin.Flag(
		"names.reverse-lookup", "When capture-client is enabled for the Names collector, perform a reverse DNS lookup to identify the client in the vector instead of the IP. ($BIND_QUERY_EXPORTER_NAMES_REVERSE_LOOKUP)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_REVERSE_LOOKUP").Default("false").Bool()

	bindQueryStatsCaptureClient = kingpin.Flag(
		"stats.capture-client", "Enable capturing the client making the client IP or name as part of the vector. WARNING: This will can lead to lots of metrics in your Prometheus database! ($BIND_QUERY_EXPORTER_STATS_CAPTURE_CLIENT)",
	).Envar("BIND_QUERY_EXPORTER_STATS_CAPTURE_CLIENT").Default("false").Bool()

	bindQueryStatsReverseLookup = kingpin.Flag(
		"stats.reverse-lookup", "When capture-client is enabled for the Stats collector, perform a reverse DNS lookup to identify the client in the vector instead of the IP. WARNING: this will create queries to your DNS server which will probably be seen by this exporter... triggering an infinite loop of lookups if you do not have a DNS cache configured!!!! ($BIND_QUERY_EXPORTER_STATS_REVERSE_LOOKUP)",
	).Envar("BIND_QUERY_EXPORTER_STATS_REVERSE_LOOKUP").Default("false").Bool()

	filterCollectors = kingpin.Flag(
		"filter.collectors", "Comma separated collectors to enable (Stats,Names) ($BIND_QUERY_EXPORTER_FILTER_COLLECTORS)",
	).Envar("BIND_QUERY_EXPORTER_FILTER_COLLECTORS").Default("Stats").String()

	metricsNamespace = kingpin.Flag(
		"metrics.namespace", "Metrics Namespace ($BIND_QUERY_EXPORTER_METRICS_NAMESPACE)",
	).Envar("BIND_QUERY_EXPORTER_METRICS_NAMESPACE").Default("bind_query").String()

	listenAddress = kingpin.Flag(
		"web.listen-address", "Address to listen on for web interface and telemetry ($BIND_QUERY_EXPORTER_WEB_LISTEN_ADDRESS)",
	).Envar("BIND_QUERY_EXPORTER_WEB_LISTEN_ADDRESS").Default(":9197").String()

	metricsPath = kingpin.Flag(
		"web.telemetry-path", "Path under which to expose Prometheus metrics ($BIND_QUERY_EXPORTER_WEB_TELEMETRY_PATH)",
	).Envar("BIND_QUERY_EXPORTER_WEB_TELEMETRY_PATH").Default("/metrics").String()

	authUsername = kingpin.Flag(
		"web.auth.username", "Username for web interface basic auth. Password is set via $BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD env variable ($BIND_QUERY_EXPORTER_WEB_AUTH_USERNAME)",
	).Envar("BIND_QUERY_EXPORTER_WEB_AUTH_USERNAME").String()
	authPassword = ""

	tlsCertFile = kingpin.Flag(
		"web.tls.cert_file", "Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate ($BIND_QUERY_EXPORTER_WEB_TLS_CERTFILE)",
	).Envar("BIND_QUERY_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	tlsKeyFile = kingpin.Flag(
		"web.tls.key_file", "Path to a file that contains the TLS private key (PEM format) ($BIND_QUERY_EXPORTER_WEB_TLS_KEYFILE)",
	).Envar("BIND_QUERY_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	bindQueryPrintMetrics = kingpin.Flag(
		"printMetrics", "Print the metrics this exporter exposes and exits. Default: false ($BIND_QUERY_EXPORTER_PRINT_METRICS)",
	).Envar("BIND_QUERY_EXPORTER_PRINT_METRICS").Default("false").Bool()
)

func init() {
	prometheus.MustRegister(version.NewCollector(*metricsNamespace))
}

type basicAuthHandler struct {
	handler  http.HandlerFunc
	username string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != h.username || password != h.password {
		log.Errorf("Invalid HTTP auth from `%s`", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
}

func prometheusHandler() http.Handler {
	handler := promhttp.Handler()

	if *authUsername != "" && authPassword != "" {
		handler = &basicAuthHandler{
			handler:  promhttp.Handler().ServeHTTP,
			username: *authUsername,
			password: authPassword,
		}
	}

	return handler
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(Version)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *bindQueryPrintMetrics {
		matcher := util.NewLogMatcher()
		/* Make a channel and function to send output along */
		var out chan *prometheus.Desc
		eatOutput := func(in <-chan *prometheus.Desc) {
			for desc := range in {
				/* Weaksauce... no direct access to the variables */
				//Desc{fqName: "the_name", help: "help text", constLabels: {}, variableLabels: []}
				tmp := desc.String()
				vals := strings.Split(tmp, `"`)
				fmt.Printf("  %s - %s\n", vals[1], vals[3])
			}
		}

		/* Interesting juggle here...
		   - Make a channel the describe function can send output to
		   - Start the printing function that consumes the output in the background
		   - Call the describe function to feed the channel (which blocks until the consume function eats a message)
		   - When the describe function exits after returning the last item, close the channel to end the background consume function
		*/
		bogusChan := make(chan string)

		fmt.Println("Stats")
		statsCollector := collectors.NewStatsCollector(*metricsNamespace, &bogusChan, &matcher, *bindQueryStatsCaptureClient)
		out = make(chan *prometheus.Desc)
		go eatOutput(out)
		statsCollector.Describe(out)
		close(out)

		fmt.Println("Names")
		namesCollector, err := collectors.NewNamesCollector(*metricsNamespace, &bogusChan, &matcher, *bindQueryIncludeFile, *bindQueryExcludeFile, *bindQueryIncludeClientsFile, *bindQueryExcludeClientsFile, *bindQueryNamesCaptureClient)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		out = make(chan *prometheus.Desc)
		go eatOutput(out)
		namesCollector.Describe(out)
		close(out)

		os.Exit(0)
	}

	log.Infoln("Starting bind_query_exporter", Version)
	authPassword = os.Getenv("BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD")

	fi, err := os.Stat(*bindQueryLogFile)
	if err != nil {
		log.Errorln("Failed to stat file:", *bindQueryLogFile, err)
		os.Exit(1)
	}

	var collectorsFilters []string
	if *filterCollectors != "" {
		collectorsFilters = strings.Split(*filterCollectors, ",")
	}
	collectorsFilter, err := filters.NewCollectorsFilter(collectorsFilters)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	var consumers []*chan string
	if collectorsFilter.Enabled(filters.NamesCollector) {
		matcher := util.LogMatcher{
			ReverseLookup: *bindQueryNamesReverseLookup,
			Regex:         regexp.MustCompile(*bindQueryPattern),
		}
		thisChannel := make(chan string)
		consumers = append(consumers, &thisChannel)
		namesCollector, err := collectors.NewNamesCollector(*metricsNamespace, &thisChannel, &matcher, *bindQueryIncludeFile, *bindQueryExcludeFile, *bindQueryIncludeClientsFile, *bindQueryExcludeClientsFile, *bindQueryNamesCaptureClient)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		prometheus.MustRegister(namesCollector)
	}
	if collectorsFilter.Enabled(filters.StatsCollector) {
		matcher := util.LogMatcher{
			ReverseLookup: *bindQueryStatsReverseLookup,
			Regex:         regexp.MustCompile(*bindQueryPattern),
		}
		thisChannel := make(chan string)
		consumers = append(consumers, &thisChannel)
		statsCollector := collectors.NewStatsCollector(*metricsNamespace, &thisChannel, &matcher, *bindQueryStatsCaptureClient)
		prometheus.MustRegister(statsCollector)
	}

	go func(*[]*chan string) {
		info := &tail.SeekInfo{Offset: fi.Size(), Whence: 0}
		t, _ := tail.TailFile(*bindQueryLogFile, tail.Config{Follow: true, ReOpen: true, Location: info})
		for line := range t.Lines {
			log.Debugln("Read: ", line)
			for _, consumer := range consumers {
				*consumer <- line.Text
			}
		}
	}(&consumers)

	handler := prometheusHandler()
	http.Handle(*metricsPath, handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>BIND Query Exporter</title></head>
             <body>
             <h1>Bind Query Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	if *tlsCertFile != "" && *tlsKeyFile != "" {
		log.Infoln("Listening TLS on", *listenAddress)
		log.Fatal(http.ListenAndServeTLS(*listenAddress, *tlsCertFile, *tlsKeyFile, nil))
	} else {
		log.Infoln("Listening on", *listenAddress)
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}
}
