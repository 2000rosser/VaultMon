package controllers

// import (
// 	"github.com/prometheus/client_golang/prometheus"
// 	"sigs.k8s.io/controller-runtime/pkg/metrics"

// 	//import log
// 	logf "sigs.k8s.io/controller-runtime/pkg/log"
// )

// var (
// 	// cpuUsage = prometheus.NewGauge(
// 	// 	prometheus.GaugeOpts{
// 	// 		Name: "cpu_usage",
// 	// 		Help: "CPU usage",
// 	// 	},
// 	// )

// 	// memUsage = prometheus.NewGauge(
// 	// 	prometheus.GaugeOpts{
// 	// 		Name: "mem_usage",
// 	// 		Help: "Memory usage",
// 	// 	},
// 	// )

// 	vaultInfo = prometheus.NewGaugeVec(
// 		prometheus.GaugeOpts{
// 			Name: "vault_info",
// 			Help: "Information about vault instances",
// 		},
// 		[]string{
// 			"vaultName",
// 			"vaultUid",
// 			"vaultNamespace",
// 			"vaultIp",
// 			// "vaultLabels",
// 			// "vaultSecrets",
// 			"vaultReplicas",
// 			// "vaultEndpoints",
// 			// "vaultStatus",
// 			// "vaultVolumes",
// 			// "vaultIngress",
// 			// "vaultCPUUsage",
// 			// "vaultMemUsage",
// 			"vaultImage",
// 		},
// 	)
// )

// func init() {
// 	// Register custom metrics with the global prometheus registry
// 	metrics.Registry.MustRegister(vaultInfo)
// 	//log that the metrics were registered
// 	logf.Log.Info("Registered metrics")

// }
