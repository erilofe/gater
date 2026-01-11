package discovery

import (
	"fmt"
	"strings"

	"github.com/pietroagazzi/gater/internal/config"
)

// ProviderType represents the type of service discovery provider.
type ProviderType string

const (
	ProviderTypeConsul ProviderType = "consul"
	// Future: ProviderTypeStatic, ProviderTypeKubernetes, ProviderTypeEtcd
)

// NewProvider creates the appropriate service discovery provider based on configuration.
func NewProvider(cfg *config.Config) (Provider, error) {
	providerType := determineProviderType(cfg)

	switch providerType {
	case ProviderTypeConsul:
		if cfg.ConsulAddress == "" {
			return nil, fmt.Errorf("CONSUL_ADDRESS is required for consul provider")
		}
		return NewConsulProvider(cfg.ConsulAddress)

	// Future cases:
	// case ProviderTypeStatic:
	//     return static.NewStaticProvider(cfg.DiscoveryRoutes)
	// case ProviderTypeKubernetes:
	//     return kubernetes.NewKubernetesProvider(cfg.KubernetesNamespace)
	// case ProviderTypeEtcd:
	//     return etcd.NewEtcdProvider(cfg.EtcdEndpoints, cfg.EtcdPrefix)

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// determineProviderType decides which provider to use based on configuration.
func determineProviderType(cfg *config.Config) ProviderType {
	// Priority:
	// 1. DISCOVERY_PROVIDER explicit setting (if set)
	// 2. Auto-detection based on available config

	if cfg.DiscoveryProvider != "" {
		return ProviderType(strings.ToLower(cfg.DiscoveryProvider))
	}

	// Auto-detection: if CONSUL_ADDRESS is present, use Consul
	if cfg.ConsulAddress != "" {
		return ProviderTypeConsul
	}

	// Future: Add auto-detection for other providers
	// if cfg.DiscoveryRoutes != "" {
	//     return ProviderTypeStatic
	// }
	// if cfg.KubernetesNamespace != "" {
	//     return ProviderTypeKubernetes
	// }

	// Default fallback (for now, requires Consul)
	return ProviderTypeConsul
}
