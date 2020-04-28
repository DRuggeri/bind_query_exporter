package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/DRuggeri/bind_query_exporter/collectors"
	"github.com/DRuggeri/bind_query_exporter/filters"
)

var (
	bindQueryLogFile = kingpin.Flag(
		"log", "Path of the BIND query log to watch. Defaults to '/var/log/bind/queries.log' ($BIND_QUERY_EXPORTER_LOG)",
	).Envar("BIND_QUERY_EXPORTER_LOG").Default("/var/log/bind/queries.log").String()

	bindQueryIncludeFile = kingpin.Flag(
		"names.include.file", "Path to a file of DNS names that this exporter WILL export when the Names filter is enabled. One DNS name per line will be read. ($BIND_QUERY_EXPORTER_NAMES_INCLUDE_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_INCLUDE_FILE").Default("").String()

	bindQueryExcludeFile = kingpin.Flag(
		"names.exclude.file", "Path to a file of DNS names that this exporter WILL NOT export when the Names filter is enabled. One DNS name per line will be read. ($BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE").Default("").String()

	bindQueryCaptureClient = kingpin.Flag(
		"names.capture-client", "Enable capturing the client making the query as part of the vector. WARNING: This will can lead to lots of metrics in your Prometheus database! ($BIND_QUERY_EXPORTER_NAMES_CAPTURE_CLIENT)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE").Bool()

	bindQueryReverseLookup = kingpin.Flag(
		"names.reverse-lookup", "When names.capture-client is enabled, enable a reverse DNS lookup to identify the client in the vector instead of the IP. ($BIND_QUERY_EXPORTER_NAMES_REVERSE_LOOKUP)",
	).Envar("BIND_QUERY_EXPORTER_NAMES_REVERSE_LOOKUP").Bool()

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

/*
	authPassword = kingpin.Flag(
		"web.auth.password", "Password for web interface basic auth ($BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD)",
	).Envar("BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD").String()
*/
	authPassword *string

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
	return
}

func prometheusHandler() http.Handler {
	handler := promhttp.Handler()

	if *authUsername != "" && *authPassword != "" {
		handler = &basicAuthHandler{
			handler:  promhttp.Handler().ServeHTTP,
			username: *authUsername,
			password: *authPassword,
		}
	}

	return handler
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("bind_query_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *bindQueryPrintMetrics {
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
		statsCollector := collectors.NewStatsCollector(*metricsNamespace, &bogusChan)
		out = make(chan *prometheus.Desc)
		go eatOutput(out)
		statsCollector.Describe(out)
		close(out)

		fmt.Println("Names")
		namesCollector, err := collectors.NewNamesCollector(*metricsNamespace, &bogusChan, *bindQueryIncludeFile, *bindQueryExcludeFile, *bindQueryCaptureClient, *bindQueryReverseLookup)
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

	log.Infoln("Starting bind_query_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
        *authPassword = os.Getenv("BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD")

	fi, err := os.Stat(*bindQueryLogFile)
	if err != nil {
		log.Errorln("Failed to stat ", bindQueryLogFile)
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

	if (*bindQueryCaptureClient || *bindQueryReverseLookup) && !collectorsFilter.Enabled(filters.NamesCollector) {
		log.Error("Error: the Names collector is not enabled, but the parameters to capture client address or perform reverse lookup is set. Did you forget to set --filter.collectors=Names?")
		os.Exit(1)
	}

	var consumers []*chan string
	if collectorsFilter.Enabled(filters.NamesCollector) {
		thisChannel := make(chan string)
		consumers = append(consumers, &thisChannel)
		namesCollector, err := collectors.NewNamesCollector(*metricsNamespace, &thisChannel, *bindQueryIncludeFile, *bindQueryExcludeFile, *bindQueryCaptureClient, *bindQueryReverseLookup)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		prometheus.MustRegister(namesCollector)
	}
	if collectorsFilter.Enabled(filters.StatsCollector) {
		thisChannel := make(chan string)
		consumers = append(consumers, &thisChannel)
		statsCollector := collectors.NewStatsCollector(*metricsNamespace, &thisChannel)
		prometheus.MustRegister(statsCollector)
	}

	go func(*[]*chan string) {
		info := &tail.SeekInfo{Offset: fi.Size(), Whence: 0}
		t, _ := tail.TailFile(*bindQueryLogFile, tail.Config{Follow: true, Location: info})
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
