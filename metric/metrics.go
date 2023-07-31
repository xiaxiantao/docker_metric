package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CpuRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "cpu_rate",
			Help:      "docker container cpu rate",
		},
		[]string{"id", "name", "image"},
	)
	MemUse = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "mem_use",
			Help:      "docker container mem use",
		},
		[]string{"id", "name", "image"},
	)
	MemLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "mem_limit",
			Help:      "docker container mem limit",
		},
		[]string{"id", "name", "image"},
	)
	MemRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "mem_rate",
			Help:      "docker container mem rate",
		},
		[]string{"id", "name", "image"},
	)
	Uptime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "up",
			Help:      "docker container up time",
		},
		[]string{"id", "name", "image", "state", "status"},
	)
	State = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "state",
			Help:      "docker container is running or not",
		},
		[]string{"id", "name", "image"},
	)
	Status = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "mesosdocker",
			Name:      "status",
			Help:      "docker container up time",
		},
		[]string{"id", "name", "image"},
	)
)

func init() {
	prometheus.MustRegister(CpuRate)
	prometheus.MustRegister(MemUse)
	prometheus.MustRegister(MemLimit)
	prometheus.MustRegister(MemRate)
	prometheus.MustRegister(Uptime)
	prometheus.MustRegister(State)
	prometheus.MustRegister(Status)
}
