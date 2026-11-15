package util

import (
	"net"
	"regexp"
	"strings"

	"github.com/prometheus/common/log"
)

var LogMatcherDefaultPattern string = `client(?: @0x[0-9a-f]+)? ([^\s#]+).*query: ([^\s]+).*IN ([^\s]+)`

type LogMatcher struct {
	//05-Jun-2021 07:24:47.780 queries: info: client @0xadfc0030 192.168.0.123#59542 (bitnebula.com): query: bitnebula.com IN A + (192.168.0.456)
	Regex         *regexp.Regexp
	ReverseLookup bool
	Include       map[string]bool
	Exclude       map[string]bool
	IncludeClient map[string]bool
	ExcludeClient map[string]bool
}

type LogMatch struct {
	Matched     bool
	QueryClient string
	QueryName   string
	QueryType   string
}

func NewLogMatcher() LogMatcher {
	return LogMatcher{
		ReverseLookup: false,
		Regex:         regexp.MustCompile(LogMatcherDefaultPattern),
	}
}

func (m LogMatcher) ExtractInfo(line string) LogMatch {
	result := LogMatch{Matched: false}

	match := m.Regex.FindStringSubmatch(line)
	if len(match) > 0 {
		result.Matched = true
		result.QueryClient = match[1]
		result.QueryName = strings.ToLower(match[2])
		result.QueryType = match[3]

		/* Check if we should avoid a DNS lookup since this name is not
		   in the list of names we care about */
		if len(m.Include) > 0 && !m.Include[result.QueryName] {
			log.Debugf("Name %s is not in include", result.QueryName)
			result.Matched = false
		}
		if len(m.Exclude) > 0 && m.Exclude[result.QueryName] {
			log.Debugf("Ignoring name %s", result.QueryName)
			result.Matched = false
		}

		if result.Matched && m.ReverseLookup {
			if names, dnsErr := net.LookupAddr(result.QueryClient); dnsErr == nil && len(names) > 0 {
				result.QueryClient = strings.TrimSuffix(names[0], ".")
			}
		}

		if len(m.IncludeClient) > 0 && !m.IncludeClient[result.QueryClient] {
			log.Debugf("Ignoring client for not being in include list: %s", result.QueryClient)
			result.Matched = false
		}
		if len(m.ExcludeClient) > 0 && m.ExcludeClient[result.QueryClient] {
			log.Debugf("Ignoring client in exclude list: %s", result.QueryClient)
			result.Matched = false
		}

		log.Debugf("Result %t for name: %s, client: %s", result.Matched, result.QueryName, result.QueryClient)
	}
	return result

}
