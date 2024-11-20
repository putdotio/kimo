package server

import (
	"strings"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetric is the type that contains all metrics those will be exposed.
type PrometheusMetric struct {
	conns  prometheus.Gauge
	conn   *prometheus.GaugeVec
	Server *Server
}

// NewPrometheusMetric is the constructor function of PrometheusMetric
func NewPrometheusMetric(server *Server) *PrometheusMetric {
	return &PrometheusMetric{
		Server: server,
		conns: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kimo_conns_total",
			Help: "Total number of db processes (conns)",
		}),
		conn: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_connection",
			Help: "Kimo connection with labels",
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

// Set sets all metrics based on Processes
func (pm *PrometheusMetric) Set() {
	// clear previous run.
	pm.conns.Set(0)
	pm.conn.MetricVec.Reset()

	ps := pm.Server.KimoProcesses
	log.Debugf("Found '%d' processes. Setting metrics...\n", len(pm.Server.KimoProcesses))

	pm.conns.Set(float64(len(ps)))

	for _, p := range ps {
		pm.conn.With(prometheus.Labels{
			"db":      p.DB,
			"host":    p.Host,
			"command": p.Command,
			"state":   p.State,
			"cmdline": strings.Join(p.CmdLine, " "),
		}).Inc()
	}
}
