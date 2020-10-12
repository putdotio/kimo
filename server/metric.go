package server

import (
	"time"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetric is the type that contains all metrics those will be exposed.
type PrometheusMetric struct {
	connCount prometheus.Gauge
	hostConn  *prometheus.GaugeVec
	dbCount   *prometheus.GaugeVec
	Server    *Server
}

// NewPrometheusMetric is the constructor function of PrometheusMetric
func NewPrometheusMetric(server *Server) *PrometheusMetric {
	return &PrometheusMetric{
		Server: server,
		connCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kimo_conn_count",
			Help: "Total number of db process",
		}),
		hostConn: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_host_conns",
			Help: "Connection count per host",
		},
			[]string{
				"host",
			},
		),
		dbCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_db_count",
			Help: "Total number of connections per db",
		},
			[]string{
				"db",
			}),
	}
}

// SetMetrics is used to set metrics periodically.
func (pm *PrometheusMetric) SetMetrics() {
	// todo: configurable time
	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		// todo: add return case
		case <-ticker.C:
			pm.Set()
		}
	}
}

// Set sets all metrics based on Processes
func (pm *PrometheusMetric) Set() {
	if len(pm.Server.Processes) == 0 {
		return
	}

	log.Debugf("Found '%d' processes. Setting metrics...\n", len(pm.Server.Processes))

	pm.connCount.Set(float64(len(pm.Server.Processes)))

	var metricM = map[string]map[string]int{}
	// todo: keys should be constant at somewhere else and we should iterate through them
	metricM["db"] = map[string]int{}
	metricM["host"] = map[string]int{}

	for _, p := range pm.Server.Processes {
		// todo: keys should be constant at somewhere else and we should iterate through them
		metricM["db"][p.DB]++
		metricM["host"][p.Host]++
	}
	for k, v := range metricM {
		if k == "db" {
			// todo: DRY
			for i, j := range v {
				pm.dbCount.With(prometheus.Labels{"db": i}).Set(float64(j))
			}
		} else {
			if k == "host" {
				// todo: DRY
				for i, j := range v {
					pm.hostConn.With(prometheus.Labels{"host": i}).Set(float64(j))
				}
			}
		}
	}
}
