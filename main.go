/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rossoperatoriov1alpha1 "github.com/2000rosser/FYP.git/api/v1alpha1"
	"github.com/2000rosser/FYP.git/controllers"

	//+kubebuilder:scaffold:imports
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(rossoperatoriov1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type CustomCollector struct {
	cpuUsage *prometheus.Desc
	memUsage *prometheus.Desc
}

func NewCustomCollector() *CustomCollector {
	return &CustomCollector{
		cpuUsage: prometheus.NewDesc("cpu_usage", "CPU usage", nil, nil),
		memUsage: prometheus.NewDesc("memory_usage", "Memory usage", nil, nil),
	}
}

func (collector *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.cpuUsage
	ch <- collector.memUsage
}

func (collector *CustomCollector) Collect(ch chan<- prometheus.Metric) {
	v, _ := mem.VirtualMemory()
	ch <- prometheus.MustNewConstMetric(collector.memUsage, prometheus.GaugeValue, v.UsedPercent)

	c, _ := cpu.Percent(0, false)
	if len(c) > 0 {
		ch <- prometheus.MustNewConstMetric(collector.cpuUsage, prometheus.GaugeValue, c[0])
	}
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var customMetricsEndpoint string
	flag.StringVar(&customMetricsEndpoint, "custom-metrics-endpoint", "/metrics", "The custom metrics endpoint path.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "38601c6c.rossoperator.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()
	// registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	// registry.MustRegister(prometheus.NewGoCollector())

	customCollector := NewCustomCollector()
	registry.MustRegister(customCollector)

	go func() {
		http.Handle(customMetricsEndpoint, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		if err := http.ListenAndServe(":8005", nil); err != nil {
			setupLog.Error(err, "metrics server failed to start")
			os.Exit(1)
		}
	}()

	vaultMetrics := controllers.NewVaultMonMetrics(registry)

	if err = (&controllers.VaultMonReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		VaultMetrics: vaultMetrics,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VaultMon")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
