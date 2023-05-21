package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
)

type VaultMonMetrics struct {
	VaultInfo     *prometheus.GaugeVec
	VaultCPUUsage *prometheus.GaugeVec
	VaultMemUsage *prometheus.GaugeVec
}

func NewVaultMonMetrics(registry *prometheus.Registry) *VaultMonMetrics {
	vaultInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vault_info",
		Help: "Information about vault instances",
	}, []string{
		"vaultName",
		"vaultUid",
		"vaultNamespace",
		"vaultIp",
		"vaultImage",
		"vaultStatus",
		"vaultReplicas",
		"vaultIngress",
		"vaultVolumes",
		"vaultEndpoints",
		"timestamp",
	})

	vaultCPUUsage := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vault_cpu_usage",
		Help: "CPU usage for Vault instances",
	}, []string{"vaultName", "vaultNamespace"})

	vaultMemUsage := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vault_mem_usage",
		Help: "Memory usage for Vault instances",
	}, []string{"vaultName", "vaultNamespace"})

	registry.MustRegister(vaultInfo, vaultCPUUsage, vaultMemUsage)

	return &VaultMonMetrics{
		VaultInfo:     vaultInfo,
		VaultCPUUsage: vaultCPUUsage,
		VaultMemUsage: vaultMemUsage,
	}
}
