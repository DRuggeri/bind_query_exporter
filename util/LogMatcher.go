package util

import (
	"github.com/prometheus/common/log"
	"net"
	"regexp"
)

var LogMatcherDefaultPattern string = `client(?: @0x[0-9a-f]+)? ([^\s#]+).*query: ([^\s]+).*IN ([^\s]+)`

type LogMatcher struct {
	//05-Jun-2021 07:24:47.780 queries: info: client @0xadfc0030 192.168.0.123#59542 (bitnebula.com): query: bitnebula.com IN A + (192.168.0.456)
	Regex         *regexp.Regexp
	ReverseLookup bool
	Include       map[string]bool
	Exclude       map[string]bool
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
		result.QueryName = match[2]
		result.QueryType = match[3]

		/* Check if we should avoid a DNS lookup since this name is not
		   in the list of names we care about */
		if len(m.Include) > 0 {
			if _, ok := m.Include[result.QueryName]; !ok {
				result.Matched = false
			}
		} else if len(m.Exclude) > 0 {
			if _, ok := m.Exclude[result.QueryName]; !ok {
				result.Matched = false
			}
		}

		if result.Matched && m.ReverseLookup {
			if names, dnsErr := net.LookupAddr(result.QueryClient); dnsErr == nil && len(names) > 0 {
				result.QueryClient = names[0]
			}
		}
	}
	log.Debugln("Found matches: ", result.Matched)
	return result

}
