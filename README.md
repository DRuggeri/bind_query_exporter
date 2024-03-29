# BIND Query log Prometheus Exporter

A [Prometheus](https://prometheus.io) exporter that captures information from the BIND queries log file. It is based on the [node_exporter](https://github.com/prometheus/node_exporter) and [cf_exporter](https://github.com/bosh-prometheus/cf_exporter) projects.

By default, this exporter's Stats collector doesn't do anything special that you can't get with the much better [bind_exporter](https://github.com/prometheus-community/bind_exporter) query stats. However, enabling the `Names` collector with `--filter.collectors="Names"` makes DNS query hits per name available (see the warning in the Names collector documentation below). This can be useful for a few use cases:
 - Using the `--names.include.file` to see if a list of DNS names you would like to decommission are still receiving queries
 - Using the `--names.include.file` to identify if clients on your network are reaching out to forbidden domain names
 - Using the `--names.exclude.file` to see if your authoritative DNS server is receiving queries for domain names you don't own

Depending on the use case, enabling `--names.capture-client` and `--reverse-lookup` may be helpful.


## BIND configuration

Note that BIND does not log queries by default, so logging must be turned on before this collector will do much. On Debian-based systems, placing the following contents in `/etc/bind/named.conf.logging` will enable logging:

```
logging {
  channel syslog { syslog daemon; severity info; };
  channel stdout { stderr; severity info; };
  channel transfer_log {
    file "/var/log/bind/bind.log" versions 10 size 50M;
    severity info;
    print-category yes;
    print-severity yes;
    print-time yes;
  };
  channel query_log {
    file "/var/log/bind/queries.log" versions 10 size 50M;
    severity debug;
    print-category yes;
    print-severity yes;
    print-time yes;
  };
  category default { syslog; stdout; };
  category update { syslog; };
  category update-security { syslog; };
  category security { syslog; };
  category queries { query_log; };
  category xfer-in { transfer_log; };
  category xfer-out { transfer_log; };
  category lame-servers { null; };
};
```

It is **strongly** suggested to enable rotation of the log file. On Debian-based systems, you can do this by creating the file `/etc/logrotate.d/bind` with these contents:

```
/var/log/bind/bind.log {
  daily
  missingok
  rotate 7
  compress
  delaycompress
  notifempty
  create 644 bind bind
}
/var/log/bind/queries.log {
  daily
  missingok
  rotate 7
  compress
  delaycompress
  notifempty
  create 644 bind bind
  postrotate
    /usr/sbin/invoke-rc.d bind9 reload > /dev/null
  endscript
}
```

## Installation

### Binaries

Download the already existing [binaries](https://github.com/DRuggeri/bind_query_exporter/releases) for your platform:

```bash
$ ./bind_query_exporter <flags>
```

### From source

Using the standard `go install` (you must have [Go](https://golang.org/) already installed in your local machine):

```bash
$ go install github.com/DRuggeri/bind_query_exporter
$ bind_query_exporter <flags>
```

### With Docker
An official scratch-based Docker image is built with every tag and pushed to DockerHub and ghcr. Additionally, PRs will be tested by GitHubs actions.

The following images are available for use:
- [druggeri/bind_query_exporter](https://hub.docker.com/r/druggeri/bind_query_exporter)
- [ghcr.io/DRuggeri/bind_query_exporter](https://ghcr.io/DRuggeri/bind_query_exporter)

## Usage

### Flags

```
usage: bind_query_exporter [<flags>]

Flags:
  -h, --help                   Show context-sensitive help (also try --help-long and --help-man).
      --log="/var/log/bind/queries.log"  
                               Path of the BIND query log to watch. Defaults to '/var/log/bind/queries.log' ($BIND_QUERY_EXPORTER_LOG)
      --pattern="client(?: @0x[0-9a-f]+)? ([^\\s#]+).*query: ([^\\s]+).*IN ([^\\s]+)"  
                               The regular expression pattern with three capturing matches for the client IP, the queried name, and the query type ($BIND_QUERY_EXPORTER_PATTERN)
      --names.include.file=""  Path to a file of DNS names that this exporter WILL export when the Names filter is enabled. One DNS name per line will be read. ($BIND_QUERY_EXPORTER_NAMES_INCLUDE_FILE)
      --names.exclude.file=""  Path to a file of DNS names that this exporter WILL NOT export when the Names filter is enabled. One DNS name per line will be read.
                               ($BIND_QUERY_EXPORTER_NAMES_EXCLUDE_FILE)
      --names.exclude-clients.file=""  
                               Path to a file of reverse names or IP addresses that this exporter will ignore. One entry per line will be read. ($BIND_QUERY_EXPORTER_NAMES_EXCLUDE_CLIENTS_FILE)
      --names.include-clients.file=""  
                               Path to a file of reverse names or IP addresses that this exporter will capture. All others will be ignored. One entry per line will be read.
                               ($BIND_QUERY_EXPORTER_NAMES_INCLUDE_CLIENTS_FILE)
      --names.capture-client   Enable capturing the client making the client IP or name as part of the vector. WARNING: This will can lead to lots of metrics in your Prometheus database!
                               ($BIND_QUERY_EXPORTER_NAMES_CAPTURE_CLIENT)
      --names.reverse-lookup   When capture-client is enabled for the Names collector, perform a reverse DNS lookup to identify the client in the vector instead of the IP.
                               ($BIND_QUERY_EXPORTER_NAMES_REVERSE_LOOKUP)
      --stats.capture-client   Enable capturing the client making the client IP or name as part of the vector. WARNING: This will can lead to lots of metrics in your Prometheus database!
                               ($BIND_QUERY_EXPORTER_STATS_CAPTURE_CLIENT)
      --stats.reverse-lookup   When capture-client is enabled for the Stats collector, perform a reverse DNS lookup to identify the client in the vector instead of the IP. WARNING: this will create
                               queries to your DNS server which will probably be seen by this exporter... triggering an infinite loop of lookups if you do not have a DNS cache configured!!!!
                               ($BIND_QUERY_EXPORTER_STATS_REVERSE_LOOKUP)
      --filter.collectors="Stats"  
                               Comma separated collectors to enable (Stats,Names) ($BIND_QUERY_EXPORTER_FILTER_COLLECTORS)
      --metrics.namespace="bind_query"  
                               Metrics Namespace ($BIND_QUERY_EXPORTER_METRICS_NAMESPACE)
      --web.listen-address=":9197"  
                               Address to listen on for web interface and telemetry ($BIND_QUERY_EXPORTER_WEB_LISTEN_ADDRESS)
      --web.telemetry-path="/metrics"  
                               Path under which to expose Prometheus metrics ($BIND_QUERY_EXPORTER_WEB_TELEMETRY_PATH)
      --web.auth.username=WEB.AUTH.USERNAME  
                               Username for web interface basic auth. Password is set via $BIND_QUERY_EXPORTER_WEB_AUTH_PASSWORD env variable ($BIND_QUERY_EXPORTER_WEB_AUTH_USERNAME)
      --web.tls.cert_file=WEB.TLS.CERT_FILE  
                               Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the
                               server's certificate, any intermediates, and the CA's certificate ($BIND_QUERY_EXPORTER_WEB_TLS_CERTFILE)
      --web.tls.key_file=WEB.TLS.KEY_FILE  
                               Path to a file that contains the TLS private key (PEM format) ($BIND_QUERY_EXPORTER_WEB_TLS_KEYFILE)
      --printMetrics           Print the metrics this exporter exposes and exits. Default: false ($BIND_QUERY_EXPORTER_PRINT_METRICS)
      --log.level="info"       Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
      --log.format="logger:stderr"  
                               Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
      --version                Show application version.
```

## Metrics

### Stats
This collector counts the number of DNS queries the DNS server receives by type. When enabled, it can break the number of DNS queries by type down by each client on the network.

**IMPORTANT NOTE** Be very careful when enabling the `--stats.reverse-lookup` option on this collector. You MUST have a caching resolver (such as dnsmasq) configured on this system.
This is because the exporter does not cache reverse lookup information.
This means each line read from the log file will result in a DNS query to look the client up.
At best, this doubles the load on your DNS server.
At worst, it will cause an infinite lookup loop where a reverse lookup triggers a log entry, which triggers a reverse lookup, which triggers a log entry.

**IMPORTANT NOTE** Consider the size of your client network before enabling the `capture-client` option.
See the note below in the Names collector for why this is a possible concern for your Prometheus installation.

```
  bind_query_stats_total - Total queries recieved
  bind_query_stats_total_by_type - Total queries recieved by type of query
  bind_query_stats_by_client_and_type - Total queries recieved by type of query by client
```

### Names
This collector counts unique hits to individual DNS names by setting the metric `bind_query_names_all{name="site.foo.bar.com",...} 123`.
If the `--names.capture-clients` flag is set, the vector will also include the address of the client (or reverse lookup with `--reverse-lookup`).

**IMPORTANT NOTE:** Each DNS name detected will gets its own label in the `bind_query_names_all` vector.
Depending on the number of things matched, you may expose yourself to the cardinality problems mentioned [here](https://prometheus.io/docs/practices/instrumentation/#do-not-overuse-labels) and [here](https://prometheus.io/docs/practices/naming/#labels) - especially if your nameserver is used as a recursive server or sees hits for many domains!
Consider using the includeFile as a permit list to limit what is gathered.
Because of this, the Names collector is not enabled by default.

```
  bind_query_names_all - Queries per DNS name
  bind_query_names_total - Sum of all queries matched. If no include/exclude filter is present, this will match bind_query_stats_total in the stats collector.  It is initialized to 0 to support increment() detection.
```

## Contributing

Refer to the [contributing guidelines](https://github.com/DRuggeri/bind_query_exporter/blob/master/CONTRIBUTING.md).

## License

Apache License 2.0, see [LICENSE](https://github.com/DRuggeri/bind_query_exporter/blob/master/LICENSE).
