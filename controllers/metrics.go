package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
)

type VaultMonMetrics struct {
	VaultInfo *prometheus.GaugeVec
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
		"timestamp",
	})

	registry.MustRegister(vaultInfo)

	return &VaultMonMetrics{
		VaultInfo: vaultInfo,
	}
}
