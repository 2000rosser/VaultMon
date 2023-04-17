package metrics

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
		"vaultReplicas",
		"vaultImage",
	})

	registry.MustRegister(vaultInfo)

	return &VaultMonMetrics{
		VaultInfo: vaultInfo,
	}
}
