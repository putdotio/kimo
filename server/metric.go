package server

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetric represents the type that contains all metrics those will be exposed.
type PrometheusMetric struct {
	conns prometheus.Gauge
	conn  *prometheus.GaugeVec

	cmdlineRegexps []*regexp.Regexp
}

// NewPrometheusMetric creates and returns a new PrometheusMetric.
func NewPrometheusMetric(cmdlinePatterns []string) *PrometheusMetric {
	return &PrometheusMetric{
		cmdlineRegexps: convertPatternsToRegexps(cmdlinePatterns),
		conns: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kimo_mysql_conns_total",
			Help: "Total number of db processes (conns)",
		}),
		conn: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_mysql_connection",
			Help: "Kimo mysql connection.",
		},
			[]string{
				"db",
				"host",
				"command",
				"state",
				"cmdline",
			},
		),
	}
}

// convertPatternsToRegexps converts given patterns into regexps.
func convertPatternsToRegexps(patterns []string) []*regexp.Regexp {
	rps := make([]*regexp.Regexp, 0)
	for _, pattern := range patterns {
		rps = append(rps, regexp.MustCompile(pattern))
	}
	return rps

}

// Set sets all metrics based on Processes
func (pm *PrometheusMetric) Set(kps []KimoProcess) {
	// clear previous run.
	pm.conns.Set(0)
	pm.conn.MetricVec.Reset()

	log.Debugf("Found '%d' processes. Setting metrics...\n", len(kps))

	pm.conns.Set(float64(len(kps)))

	for _, p := range kps {
		pm.conn.With(prometheus.Labels{
			"db":      p.DB,
			"host":    p.Host,
			"command": p.Command,
			"state":   p.State,
			"cmdline": pm.formatCmdline(p.CmdLine),
		}).Inc()
	}
}

// formatCmdline formats the command string based on configuration
func (pm *PrometheusMetric) formatCmdline(cmdline string) string {
	// Expose whole cmdline if pattern matches.
	for _, cmdlineRegexp := range pm.cmdlineRegexps {
		result := cmdlineRegexp.FindString(cmdline)
		if result != "" {
			return cmdline
		}
	}
	// anonymize cmdline
	parts := strings.Split(cmdline, " ")
	if len(parts) >= 2 {
		return fmt.Sprintf("%s %s <params>", parts[0], parts[1])
	}
	return parts[0]
}
