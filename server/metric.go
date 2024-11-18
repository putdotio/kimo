package server

import (
	"strings"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetric is the type that contains all metrics those will be exposed.
type PrometheusMetric struct {
	conns   prometheus.Gauge
	host    *prometheus.GaugeVec
	db      *prometheus.GaugeVec
	command *prometheus.GaugeVec
	state   *prometheus.GaugeVec
	cmdline *prometheus.GaugeVec
	Server  *Server
}

// NewPrometheusMetric is the constructor function of PrometheusMetric
func NewPrometheusMetric(server *Server) *PrometheusMetric {
	return &PrometheusMetric{
		Server: server,
		conns: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "kimo_conns_total",
			Help: "Total number of db processes (conns)",
		}),
		host: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_conns_host",
			Help: "Conns per host",
		},
			[]string{
				"host",
			},
		),
		db: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_conns_db",
			Help: "Conns per db",
		},
			[]string{
				"db",
			},
		),
		command: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_conns_command",
			Help: "Conns per command",
		},
			[]string{
				"command",
			},
		),
		state: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_conns_state",
			Help: "Conns per state",
		},
			[]string{
				"state",
			},
		),
		cmdline: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kimo_conns_cmdline",
			Help: "conns per cmdline",
		},
			[]string{
				"cmdline",
			},
		),
	}
}

// Set sets all metrics based on Processes
func (pm *PrometheusMetric) Set() {
	ps := pm.Server.KimoProcesses
	if len(ps) == 0 {
		return
	}
	log.Debugf("Found '%d' processes. Setting metrics...\n", len(pm.Server.KimoProcesses))
	pm.conns.Set(float64(len(ps)))

	// todo: too much duplication
	var metricM = map[string]map[string]int{}
	metricM["db"] = map[string]int{}
	metricM["host"] = map[string]int{}
	metricM["state"] = map[string]int{}
	metricM["command"] = map[string]int{}
	metricM["cmdline"] = map[string]int{}

	for _, p := range ps {
		metricM["db"][p.DB]++
		metricM["host"][p.Host]++
		metricM["command"][p.Command]++
		metricM["state"][p.State]++
		metricM["cmdline"][strings.Join(p.CmdLine, " ")]++
	}
	for k, v := range metricM {
		switch k {
		case "db":
			setGaugeVec(k, v, pm.db)
		case "host":
			setGaugeVec(k, v, pm.host)
		case "command":
			setGaugeVec(k, v, pm.command)
		case "state":
			setGaugeVec(k, v, pm.state)
		case "cmdline":
			setGaugeVec(k, v, pm.cmdline)
		}
	}
}

func setGaugeVec(name string, m map[string]int, gv *prometheus.GaugeVec) {
	for i, j := range m {
		if i == "" {
			i = "UNKNOWN"
		}
		gv.With(prometheus.Labels{name: i}).Set(float64(j))
	}
}
