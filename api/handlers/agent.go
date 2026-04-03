package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AgentHandler handles agent status reporting.
type AgentHandler struct {
	DB *sql.DB
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(db *sql.DB) *AgentHandler {
	return &AgentHandler{DB: db}
}

// Report receives a FluxReport from a cluster agent and upserts cluster_status.
func (h *AgentHandler) Report(c echo.Context) error {
	clusterID := c.Get("cluster_id").(string)

	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20)) // 1MB limit
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to read body")
	}

	// Validate it's valid JSON.
	var fluxReport json.RawMessage
	if err := json.Unmarshal(body, &fluxReport); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid JSON")
	}

	// Extract summary fields from the FluxReport.
	summary := extractFluxSummary(fluxReport)
	now := time.Now().UTC()

	// Upsert: update if exists, insert if not.
	var existingID string
	err = h.DB.QueryRow(`SELECT id FROM cluster_status WHERE cluster_id = $1`, clusterID).Scan(&existingID)
	if err == sql.ErrNoRows {
		id := uuid.New().String()
		_, err = h.DB.Exec(
			`INSERT INTO cluster_status (id, cluster_id, flux_report, last_seen_at, sync_ready, sync_revision,
			 components_ok, components_total, helmreleases_running, helmreleases_failing,
			 kustomizations_running, kustomizations_failing, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			id, clusterID, string(fluxReport), now, summary.SyncReady, summary.SyncRevision,
			summary.ComponentsOK, summary.ComponentsTotal, summary.HelmreleasesRunning, summary.HelmreleasesFailing,
			summary.KustomizationsRunning, summary.KustomizationsFailing, now)
	} else if err == nil {
		_, err = h.DB.Exec(
			`UPDATE cluster_status SET flux_report = $1, last_seen_at = $2, sync_ready = $3, sync_revision = $4,
			 components_ok = $5, components_total = $6, helmreleases_running = $7, helmreleases_failing = $8,
			 kustomizations_running = $9, kustomizations_failing = $10, updated_at = $11
			 WHERE id = $12`,
			string(fluxReport), now, summary.SyncReady, summary.SyncRevision,
			summary.ComponentsOK, summary.ComponentsTotal, summary.HelmreleasesRunning, summary.HelmreleasesFailing,
			summary.KustomizationsRunning, summary.KustomizationsFailing, now, existingID)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update status")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type fluxSummary struct {
	SyncReady              bool
	SyncRevision           string
	ComponentsOK           int
	ComponentsTotal        int
	HelmreleasesRunning    int
	HelmreleasesFailing    int
	KustomizationsRunning  int
	KustomizationsFailing  int
}

func extractFluxSummary(data json.RawMessage) fluxSummary {
	var report map[string]interface{}
	if err := json.Unmarshal(data, &report); err != nil {
		return fluxSummary{}
	}

	summary := fluxSummary{}

	spec, ok := report["spec"].(map[string]interface{})
	if !ok {
		return summary
	}

	// Components.
	if components, ok := spec["componentStatus"].([]interface{}); ok {
		summary.ComponentsTotal = len(components)
		for _, c := range components {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if status, _ := cm["status"].(string); status == "True" {
				summary.ComponentsOK++
			}
		}
	}

	// Sync status.
	if sync, ok := spec["syncStatus"].(map[string]interface{}); ok {
		if status, _ := sync["status"].(string); status == "True" {
			summary.SyncReady = true
		}
		if rev, ok := sync["revision"].(string); ok {
			summary.SyncRevision = rev
		}
	}

	// Reconcilers.
	if reconcilers, ok := spec["reconcilerStatus"].([]interface{}); ok {
		for _, r := range reconcilers {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			kind, _ := rm["kind"].(string)
			running := intFromJSON(rm, "running")
			failing := intFromJSON(rm, "failing")

			switch kind {
			case "HelmRelease":
				summary.HelmreleasesRunning = running
				summary.HelmreleasesFailing = failing
			case "Kustomization":
				summary.KustomizationsRunning = running
				summary.KustomizationsFailing = failing
			}
		}
	}

	return summary
}

func intFromJSON(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
