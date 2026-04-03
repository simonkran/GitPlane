// Package generator produces Flux CD manifests from a PlatformConfig and
// service Catalog. All functions are pure -- they return data and never
// perform I/O.
package generator

import (
	"fmt"
	"strings"

	"github.com/simonkran/gitplane/pkg/catalog"
	"github.com/simonkran/gitplane/pkg/config"
)

// Generate produces a map of file paths to YAML content for every enabled
// service in the given config. The output includes per-service HelmRelease
// and Kustomization manifests plus a root kustomization.yaml.
func Generate(cfg *config.PlatformConfig, cat *catalog.Catalog) (map[string]string, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Collect enabled service names.
	var enabled []string
	for name, svc := range cfg.Services {
		if svc.Enabled {
			enabled = append(enabled, name)
		}
	}

	if len(enabled) == 0 {
		return nil, fmt.Errorf("no services enabled")
	}

	ordered, err := catalog.ResolveDependencyOrder(cat, enabled)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	out := make(map[string]string)
	var kustomizeResources []string

	for _, name := range ordered {
		svc := cat.GetService(name)
		if svc == nil {
			return nil, fmt.Errorf("service %q not found in catalog", name)
		}

		svcCfg := cfg.Services[name]
		basePath := fmt.Sprintf("%s/%s", cfg.GitPath, name)

		// Merge default values for the cluster size with user overrides.
		values := mergeValues(svc.DefaultValues[string(cfg.ClusterSize)], svcCfg.Values)

		// HelmRelease manifest.
		hr := renderHelmRelease(cfg, svc, values)
		out[basePath+"/helmrelease.yaml"] = hr

		// Per-service Kustomization manifest (Flux Kustomization CRD).
		ks := renderFluxKustomization(cfg, name, basePath)
		out[basePath+"/kustomization.yaml"] = ks

		kustomizeResources = append(kustomizeResources, name)
	}

	// Root kustomization.yaml that references all service directories.
	out[cfg.GitPath+"/kustomization.yaml"] = renderRootKustomization(kustomizeResources)

	return out, nil
}

// mergeValues returns defaults overridden by any user-supplied values.
func mergeValues(defaults, overrides map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// renderHelmRelease produces a Flux HelmRelease CRD YAML string.
func renderHelmRelease(cfg *config.PlatformConfig, svc *catalog.Service, values map[string]interface{}) string {
	var valuesYAML string
	if len(values) > 0 {
		var lines []string
		for k, v := range values {
			lines = append(lines, fmt.Sprintf("      %s: %v", k, v))
		}
		valuesYAML = "  values:\n" + strings.Join(lines, "\n")
	}

	var dependsOn string
	if len(svc.Dependencies) > 0 {
		var deps []string
		for _, dep := range svc.Dependencies {
			deps = append(deps, fmt.Sprintf("    - name: %s", dep))
		}
		dependsOn = "  dependsOn:\n" + strings.Join(deps, "\n") + "\n"
	}

	return fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: flux-system
  labels:
    gitplane.io/org: %q
    gitplane.io/cluster: %q
    gitplane.io/stage: %q
spec:
  interval: 10m
  chart:
    spec:
      chart: %s
      version: %q
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: flux-system
%s%s`,
		svc.Name,
		cfg.OrgID,
		cfg.ClusterID,
		string(cfg.Stage),
		svc.HelmChart,
		svc.Version,
		svc.Name,
		dependsOn,
		valuesYAML,
	)
}

// renderFluxKustomization produces a Flux Kustomization CRD that points at the
// service directory inside the Git repository.
func renderFluxKustomization(cfg *config.PlatformConfig, name, path string) string {
	return fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 10m
  path: ./%s
  prune: true
  sourceRef:
    kind: GitRepository
    name: gitplane
    namespace: flux-system
`, name, path)
}

// renderRootKustomization produces a plain Kustomize kustomization.yaml that
// lists every service directory as a resource.
func renderRootKustomization(services []string) string {
	var sb strings.Builder
	sb.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n")
	for _, svc := range services {
		sb.WriteString(fmt.Sprintf("  - %s\n", svc))
	}
	return sb.String()
}
