package filters

import (
	"errors"
	"fmt"
	"strings"
)

const (
	SitesCollector = "Sites"
	StatsCollector = "Stats"
)

type CollectorsFilter struct {
	collectorsEnabled map[string]bool
}

func NewCollectorsFilter(filters []string) (*CollectorsFilter, error) {
	collectorsEnabled := make(map[string]bool)

	for _, collectorName := range filters {
		switch strings.Trim(collectorName, " ") {
		case SitesCollector:
			collectorsEnabled[SitesCollector] = true
		case StatsCollector:
			collectorsEnabled[StatsCollector] = true
		default:
			return &CollectorsFilter{}, errors.New(fmt.Sprintf("Collector filter `%s` is not supported", collectorName))
		}
	}

	return &CollectorsFilter{collectorsEnabled: collectorsEnabled}, nil
}

func (f *CollectorsFilter) Enabled(collectorName string) bool {
	if len(f.collectorsEnabled) == 0 {
		return true
	}

	if f.collectorsEnabled[collectorName] {
		return true
	}

	return false
}
