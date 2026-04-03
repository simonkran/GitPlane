package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/simonkran/gitplane/api/middleware"
	"github.com/simonkran/gitplane/pkg/catalog"
	"github.com/simonkran/gitplane/pkg/config"
	"github.com/simonkran/gitplane/pkg/generator"
)

// GenerationHandler groups manifest generation endpoints.
type GenerationHandler struct {
	DB *sql.DB
}

// NewGenerationHandler creates a new GenerationHandler.
func NewGenerationHandler(db *sql.DB) *GenerationHandler {
	return &GenerationHandler{DB: db}
}

// Generate triggers manifest generation and records it in history.
func (h *GenerationHandler) Generate(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	cfg, err := h.buildConfig(clusterID, claims.OrgID)
	if err != nil {
		return err
	}

	cat := catalog.GetCatalog()
	manifests, err := generator.Generate(cfg, cat)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "generation failed: "+err.Error())
	}

	manifestsJSON, _ := json.Marshal(manifests)
	genID := uuid.New().String()

	// Record in generation_history as pending (git commit would happen async).
	_, dbErr := h.DB.Exec(
		`INSERT INTO generation_history (id, cluster_id, triggered_by, status, manifests_json, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		genID, clusterID, claims.UserID, "pending", manifestsJSON, time.Now().UTC())
	if dbErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to record generation")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"generationId": genID,
		"status":       "pending",
		"fileCount":    len(manifests),
	})
}

// Preview returns generated manifests without committing.
func (h *GenerationHandler) Preview(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	cfg, err := h.buildConfig(clusterID, claims.OrgID)
	if err != nil {
		return err
	}

	cat := catalog.GetCatalog()
	manifests, err := generator.Generate(cfg, cat)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "generation failed: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"manifests": manifests,
		"fileCount": len(manifests),
	})
}

// GetGeneration returns the status of a specific generation run.
func (h *GenerationHandler) GetGeneration(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")
	genID := c.Param("gen_id")

	// Verify cluster belongs to org.
	var exists bool
	if err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1 AND org_id = $2)`, clusterID, claims.OrgID).Scan(&exists); err != nil || !exists {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	var gen struct {
		ID            string          `json:"id"`
		ClusterID     string          `json:"clusterId"`
		TriggeredBy   *string         `json:"triggeredBy,omitempty"`
		GitCommitSHA  *string         `json:"gitCommitSha,omitempty"`
		Status        string          `json:"status"`
		ErrorMessage  *string         `json:"errorMessage,omitempty"`
		ManifestsJSON json.RawMessage `json:"manifests,omitempty"`
		CreatedAt     time.Time       `json:"createdAt"`
	}

	var manifestsRaw []byte
	err := h.DB.QueryRow(
		`SELECT id, cluster_id, triggered_by, git_commit_sha, status, error_message, manifests_json, created_at
		 FROM generation_history WHERE id = $1 AND cluster_id = $2`, genID, clusterID,
	).Scan(&gen.ID, &gen.ClusterID, &gen.TriggeredBy, &gen.GitCommitSHA, &gen.Status, &gen.ErrorMessage, &manifestsRaw, &gen.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "generation not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	if manifestsRaw != nil {
		gen.ManifestsJSON = json.RawMessage(manifestsRaw)
	}

	return c.JSON(http.StatusOK, gen)
}

// buildConfig constructs a PlatformConfig from the cluster and its enabled services.
func (h *GenerationHandler) buildConfig(clusterID, orgID string) (*config.PlatformConfig, error) {
	var name, stage, clusterType, dnsName, clusterSize, gitPath string
	err := h.DB.QueryRow(
		`SELECT name, stage, type, dns_name, cluster_size, git_path FROM clusters WHERE id = $1 AND org_id = $2`,
		clusterID, orgID,
	).Scan(&name, &stage, &clusterType, &dnsName, &clusterSize, &gitPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, echo.NewHTTPError(http.StatusNotFound, "cluster not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	// Get enabled services.
	rows, err := h.DB.Query(
		`SELECT service_name, config_json FROM cluster_services WHERE cluster_id = $1 AND status = 'enabled'`,
		clusterID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer rows.Close()

	services := make(map[string]config.ServiceConfig)
	for rows.Next() {
		var svcName string
		var configRaw []byte
		if err := rows.Scan(&svcName, &configRaw); err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "scan error")
		}

		svcCfg := config.ServiceConfig{Enabled: true}
		if configRaw != nil && string(configRaw) != "null" {
			var values map[string]interface{}
			if err := json.Unmarshal(configRaw, &values); err == nil {
				svcCfg.Values = values
			}
		}
		services[svcName] = svcCfg
	}

	cfg := &config.PlatformConfig{
		OrgID:       orgID,
		ClusterID:   clusterID,
		ClusterName: name,
		Stage:       config.Stage(stage),
		Type:        config.ClusterType(clusterType),
		DNSName:     dnsName,
		ClusterSize: config.ClusterSize(clusterSize),
		GitPath:     gitPath,
		Services:    services,
	}

	return cfg, nil
}
