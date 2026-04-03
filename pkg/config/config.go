// Package config provides platform configuration models and validation
// for multi-tenant Kubernetes cluster management.
package config

import (
	"fmt"
	"strings"
)

// Stage represents the deployment stage of a cluster.
type Stage string

const (
	StageDev     Stage = "dev"
	StageStaging Stage = "staging"
	StageProd    Stage = "prod"
)

// ClusterType represents the role of a cluster.
type ClusterType string

const (
	TypeControlplane ClusterType = "controlplane"
	TypeWorker       ClusterType = "worker"
)

// ClusterSize represents the sizing tier of a cluster.
type ClusterSize string

const (
	SizeSmall  ClusterSize = "small"
	SizeMedium ClusterSize = "medium"
	SizeLarge  ClusterSize = "large"
)

// ServiceConfig holds per-service configuration within a platform config.
type ServiceConfig struct {
	Enabled bool                   `json:"enabled" yaml:"enabled"`
	Values  map[string]interface{} `json:"values,omitempty" yaml:"values,omitempty"`
}

// PlatformConfig is the top-level, multi-tenant-aware configuration for a
// managed Kubernetes cluster.
type PlatformConfig struct {
	OrgID       string                   `json:"orgId" yaml:"orgId"`
	ClusterID   string                   `json:"clusterId" yaml:"clusterId"`
	ClusterName string                   `json:"clusterName" yaml:"clusterName"`
	Stage       Stage                    `json:"stage" yaml:"stage"`
	Type        ClusterType              `json:"type" yaml:"type"`
	DNSName     string                   `json:"dnsName" yaml:"dnsName"`
	ClusterSize ClusterSize              `json:"clusterSize" yaml:"clusterSize"`
	Services    map[string]ServiceConfig `json:"services" yaml:"services"`
	GitPath     string                   `json:"gitPath" yaml:"gitPath"`
}

// Validate checks that all required fields are set and contain valid values.
func (c *PlatformConfig) Validate() error {
	var errs []string

	if c.OrgID == "" {
		errs = append(errs, "orgId is required")
	}
	if c.ClusterID == "" {
		errs = append(errs, "clusterId is required")
	}
	if c.ClusterName == "" {
		errs = append(errs, "clusterName is required")
	}

	switch c.Stage {
	case StageDev, StageStaging, StageProd:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("stage must be one of dev, staging, prod; got %q", c.Stage))
	}

	switch c.Type {
	case TypeControlplane, TypeWorker:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("type must be one of controlplane, worker; got %q", c.Type))
	}

	if c.DNSName == "" {
		errs = append(errs, "dnsName is required")
	}

	switch c.ClusterSize {
	case SizeSmall, SizeMedium, SizeLarge:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("clusterSize must be one of small, medium, large; got %q", c.ClusterSize))
	}

	if c.GitPath == "" {
		errs = append(errs, "gitPath is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// DefaultConfig returns a PlatformConfig populated with sensible defaults.
// OrgID and ClusterID must still be set by the caller.
func DefaultConfig() *PlatformConfig {
	return &PlatformConfig{
		Stage:       StageDev,
		Type:        TypeWorker,
		ClusterSize: SizeSmall,
		ClusterName: "default",
		DNSName:     "cluster.local",
		GitPath:     "clusters/default",
		Services:    make(map[string]ServiceConfig),
	}
}
