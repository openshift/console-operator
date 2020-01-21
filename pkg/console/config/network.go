package config

import (
	"net"

	configv1 "github.com/openshift/api/config/v1"
)

type IPMode string

const (
	IPv4Mode   IPMode = "v4"
	IPv6Mode   IPMode = "v6"
	IPv4v6Mode IPMode = "v4v6"
)

// borrowing existing logic from cluster-ingress-operator
// - https://github.com/openshift/cluster-ingress-operator/blob/master/pkg/operator/controller/ingress/deployment.go#L412
func NetworkSupported(networkConfig *configv1.Network) (IPv4IPv6mode IPMode, usingIPv4 bool, usingIPv6 bool) {
	usingIPv4 = false
	usingIPv6 = false
	mode := IPv4Mode

	for _, clusterNetworkEntry := range networkConfig.Status.ClusterNetwork {
		addr, _, err := net.ParseCIDR(clusterNetworkEntry.CIDR)
		if err != nil {
			continue
		}
		if addr.To4() != nil {
			usingIPv4 = true
		} else {
			usingIPv6 = true
		}
	}

	if usingIPv6 {
		mode = IPv4v6Mode
		if !usingIPv4 {
			mode = IPv6Mode
		}
	}
	return mode, usingIPv4, usingIPv6
}
