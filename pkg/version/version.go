package version

import (
	"fmt"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"

	info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "torarr_info",
			Help: "Information about the Torarr build",
		},
		[]string{"version", "commit", "date", "go_version"},
	)
)

func init() {
	info.WithLabelValues(Version, Commit, Date, runtime.Version()).Set(1)
}

func String() string {
	return fmt.Sprintf("Torarr %s (commit: %s, built: %s)", Version, Commit, Date)
}
