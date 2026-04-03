// Package catalog defines the curated service catalog for GitPlane-managed
// Kubernetes clusters and provides dependency resolution utilities.
package catalog

import "fmt"

// Service describes a single installable platform service.
type Service struct {
	Name         string                                  `json:"name" yaml:"name"`
	Description  string                                  `json:"description" yaml:"description"`
	Category     string                                  `json:"category" yaml:"category"`
	HelmRepo     string                                  `json:"helmRepo" yaml:"helmRepo"`
	HelmChart    string                                  `json:"helmChart" yaml:"helmChart"`
	Version      string                                  `json:"version" yaml:"version"`
	Dependencies []string                                `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	DefaultValues map[string]map[string]interface{}       `json:"defaultValues,omitempty" yaml:"defaultValues,omitempty"` // keyed by cluster size
	Enabled      bool                                    `json:"enabled" yaml:"enabled"`
}

// Catalog is an ordered collection of services.
type Catalog struct {
	Services []Service `json:"services" yaml:"services"`
}

// GetService returns a pointer to the named service, or nil if not found.
func (c *Catalog) GetService(name string) *Service {
	for i := range c.Services {
		if c.Services[i].Name == name {
			return &c.Services[i]
		}
	}
	return nil
}

// GetCatalog returns the built-in default catalog.
func GetCatalog() *Catalog {
	return &Catalog{
		Services: []Service{
			{
				Name:        "flux-operator",
				Description: "Flux GitOps operator for continuous delivery",
				Category:    "gitops",
				HelmRepo:    "https://fluxcd-community.github.io/helm-charts",
				HelmChart:   "flux-operator",
				Version:     "2.3.0",
				Enabled:     true,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"resources.limits.memory": "256Mi", "resources.limits.cpu": "250m"},
					"medium": {"resources.limits.memory": "512Mi", "resources.limits.cpu": "500m"},
					"large":  {"resources.limits.memory": "1Gi", "resources.limits.cpu": "1000m"},
				},
			},
			{
				Name:        "cert-manager",
				Description: "X.509 certificate management for Kubernetes",
				Category:    "security",
				HelmRepo:    "https://charts.jetstack.io",
				HelmChart:   "cert-manager",
				Version:     "1.15.1",
				Enabled:     true,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"replicaCount": 1},
					"medium": {"replicaCount": 2},
					"large":  {"replicaCount": 3},
				},
			},
			{
				Name:         "traefik",
				Description:  "Cloud-native ingress controller",
				Category:     "networking",
				HelmRepo:     "https://traefik.github.io/charts",
				HelmChart:    "traefik",
				Version:      "28.3.0",
				Dependencies: []string{"cert-manager"},
				Enabled:      true,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"replicas": 1},
					"medium": {"replicas": 2},
					"large":  {"replicas": 3},
				},
			},
			{
				Name:        "kube-prometheus-stack",
				Description: "Full cluster monitoring with Prometheus and Grafana",
				Category:    "observability",
				HelmRepo:    "https://prometheus-community.github.io/helm-charts",
				HelmChart:   "kube-prometheus-stack",
				Version:     "61.2.0",
				Enabled:     true,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"prometheus.retention": "7d", "grafana.replicas": 1},
					"medium": {"prometheus.retention": "15d", "grafana.replicas": 1},
					"large":  {"prometheus.retention": "30d", "grafana.replicas": 2},
				},
			},
			{
				Name:        "external-dns",
				Description: "Automatic DNS record management",
				Category:    "networking",
				HelmRepo:    "https://kubernetes-sigs.github.io/external-dns",
				HelmChart:   "external-dns",
				Version:     "1.14.5",
				Enabled:     false,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"replicas": 1},
					"medium": {"replicas": 1},
					"large":  {"replicas": 2},
				},
			},
			{
				Name:        "external-secrets",
				Description: "Synchronize secrets from external stores",
				Category:    "security",
				HelmRepo:    "https://charts.external-secrets.io",
				HelmChart:   "external-secrets",
				Version:     "0.10.0",
				Enabled:     false,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"replicaCount": 1},
					"medium": {"replicaCount": 2},
					"large":  {"replicaCount": 3},
				},
			},
			{
				Name:        "velero",
				Description: "Backup and disaster recovery for Kubernetes",
				Category:    "backup",
				HelmRepo:    "https://vmware-tanzu.github.io/helm-charts",
				HelmChart:   "velero",
				Version:     "7.1.0",
				Enabled:     false,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"resources.limits.memory": "256Mi"},
					"medium": {"resources.limits.memory": "512Mi"},
					"large":  {"resources.limits.memory": "1Gi"},
				},
			},
			{
				Name:        "reloader",
				Description: "Auto-restart workloads on ConfigMap or Secret changes",
				Category:    "operations",
				HelmRepo:    "https://stakater.github.io/stakater-charts",
				HelmChart:   "reloader",
				Version:     "1.0.115",
				Enabled:     true,
				DefaultValues: map[string]map[string]interface{}{
					"small":  {"replicaCount": 1},
					"medium": {"replicaCount": 1},
					"large":  {"replicaCount": 2},
				},
			},
		},
	}
}

// ValidateDependencies checks that every dependency of the enabled services is
// also present in the enabled list. Returns an error describing all missing
// dependencies.
func ValidateDependencies(cat *Catalog, enabled []string) error {
	set := make(map[string]struct{}, len(enabled))
	for _, name := range enabled {
		set[name] = struct{}{}
	}

	for _, name := range enabled {
		svc := cat.GetService(name)
		if svc == nil {
			return fmt.Errorf("service %q not found in catalog", name)
		}
		for _, dep := range svc.Dependencies {
			if _, ok := set[dep]; !ok {
				return fmt.Errorf("service %q requires %q, but it is not enabled", name, dep)
			}
		}
	}
	return nil
}

// ResolveDependencyOrder returns the enabled services sorted in dependency
// order (dependencies first). Returns an error on cycles or missing services.
func ResolveDependencyOrder(cat *Catalog, enabled []string) ([]string, error) {
	if err := ValidateDependencies(cat, enabled); err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(enabled))
	for _, name := range enabled {
		set[name] = struct{}{}
	}

	// Kahn's algorithm for topological sort.
	inDegree := make(map[string]int, len(enabled))
	dependents := make(map[string][]string) // dep -> list of services that depend on it

	for _, name := range enabled {
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
		svc := cat.GetService(name)
		for _, dep := range svc.Dependencies {
			if _, ok := set[dep]; ok {
				inDegree[name]++
				dependents[dep] = append(dependents[dep], name)
			}
		}
	}

	var queue []string
	for _, name := range enabled {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ordered = append(ordered, cur)
		for _, dep := range dependents[cur] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(ordered) != len(enabled) {
		return nil, fmt.Errorf("dependency cycle detected among enabled services")
	}
	return ordered, nil
}
