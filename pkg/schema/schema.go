// Package schema generates a JSON Schema representation of PlatformConfig,
// suitable for serving to a frontend for dynamic form generation.
package schema

// GenerateConfigSchema returns a JSON Schema (draft-07) that describes
// the PlatformConfig structure.
func GenerateConfigSchema() map[string]interface{} {
	serviceConfigSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the service is enabled",
			},
			"values": map[string]interface{}{
				"type":                 "object",
				"description":         "Custom Helm values to override defaults",
				"additionalProperties": true,
			},
		},
		"required": []string{"enabled"},
	}

	return map[string]interface{}{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"title":                "PlatformConfig",
		"description":         "GitPlane platform configuration for a managed Kubernetes cluster",
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"orgId", "clusterId", "clusterName", "stage", "type",
			"dnsName", "clusterSize", "gitPath",
		},
		"properties": map[string]interface{}{
			"orgId": map[string]interface{}{
				"type":        "string",
				"description": "Organization identifier for multi-tenant isolation",
				"minLength":   1,
			},
			"clusterId": map[string]interface{}{
				"type":        "string",
				"description": "Unique cluster identifier",
				"minLength":   1,
			},
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable cluster name",
				"minLength":   1,
			},
			"stage": map[string]interface{}{
				"type":        "string",
				"description": "Deployment stage",
				"enum":        []string{"dev", "staging", "prod"},
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Cluster role",
				"enum":        []string{"controlplane", "worker"},
			},
			"dnsName": map[string]interface{}{
				"type":        "string",
				"description": "Base DNS name for the cluster",
				"minLength":   1,
			},
			"clusterSize": map[string]interface{}{
				"type":        "string",
				"description": "Sizing tier that influences resource defaults",
				"enum":        []string{"small", "medium", "large"},
			},
			"services": map[string]interface{}{
				"type":                 "object",
				"description":         "Map of service name to its configuration",
				"additionalProperties": serviceConfigSchema,
			},
			"gitPath": map[string]interface{}{
				"type":        "string",
				"description": "Path within the Git repository for generated manifests",
				"minLength":   1,
			},
		},
	}
}
