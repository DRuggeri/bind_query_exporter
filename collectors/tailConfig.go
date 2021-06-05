package collectors

import (
	"github.com/DRuggeri/bind_query_exporter/util"
)

type tailConfig struct {
	matcher       *util.LogMatcher
	captureClient bool
}
