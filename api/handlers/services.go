package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/simonkran/gitplane/api/middleware"
	"github.com/simonkran/gitplane/pkg/catalog"
)

// ServiceHandler groups service catalog endpoints.
type ServiceHandler struct {
	DB *sql.DB
}

// NewServiceHandler creates a new ServiceHandler.
func NewServiceHandler(db *sql.DB) *ServiceHandler {
	return &ServiceHandler{DB: db}
}

// Catalog returns the full curated service catalog.
func (h *ServiceHandler) Catalog(c echo.Context) error {
	cat := catalog.GetCatalog()
	return c.JSON(http.StatusOK, cat)
}

type serviceEntry struct {
	ServiceName string          `json:"serviceName"`
	Status      string          `json:"status"`
	ConfigJSON  json.RawMessage `json:"configJson,omitempty"`
	CatalogInfo *catalog.Service `json:"catalogInfo,omitempty"`
}

// ListClusterServices lists services for a cluster with their enabled/disabled status.
func (h *ServiceHandler) ListClusterServices(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	// Verify cluster belongs to org.
	var exists bool
	if err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1 AND org_id = $2)`, clusterID, claims.OrgID).Scan(&exists); err != nil || !exists {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	cat := catalog.GetCatalog()

	rows, err := h.DB.Query(
		`SELECT service_name, status, config_json FROM cluster_services WHERE cluster_id = $1`,
		clusterID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer rows.Close()

	configured := make(map[string]serviceEntry)
	for rows.Next() {
		var se serviceEntry
		var configRaw []byte
		if err := rows.Scan(&se.ServiceName, &se.Status, &configRaw); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "scan error")
		}
		if configRaw != nil {
			se.ConfigJSON = json.RawMessage(configRaw)
		}
		configured[se.ServiceName] = se
	}

	// Build response: all catalog services with their cluster status.
	var services []serviceEntry
	for _, svc := range cat.Services {
		entry := serviceEntry{
			ServiceName: svc.Name,
			Status:      "disabled",
			CatalogInfo: &svc,
		}
		if cfg, ok := configured[svc.Name]; ok {
			entry.Status = cfg.Status
			entry.ConfigJSON = cfg.ConfigJSON
		}
		services = append(services, entry)
	}

	return c.JSON(http.StatusOK, services)
}

type updateServiceRequest struct {
	Status     string          `json:"status"`
	ConfigJSON json.RawMessage `json:"configJson,omitempty"`
}

// UpdateClusterService enables or disables a service for a cluster.
func (h *ServiceHandler) UpdateClusterService(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")
	serviceName := c.Param("name")

	// Verify cluster belongs to org.
	var exists bool
	if err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1 AND org_id = $2)`, clusterID, claims.OrgID).Scan(&exists); err != nil || !exists {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	// Verify service exists in catalog.
	cat := catalog.GetCatalog()
	svc := cat.GetService(serviceName)
	if svc == nil {
		return echo.NewHTTPError(http.StatusNotFound, "service not found in catalog")
	}

	var req updateServiceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Status != "enabled" && req.Status != "disabled" {
		return echo.NewHTTPError(http.StatusBadRequest, "status must be 'enabled' or 'disabled'")
	}

	// If enabling, validate dependencies are met.
	if req.Status == "enabled" {
		enabledServices, err := h.getEnabledServices(clusterID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "database error")
		}
		enabledServices = append(enabledServices, serviceName)
		if err := catalog.ValidateDependencies(cat, enabledServices); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	}

	configJSON := "null"
	if req.ConfigJSON != nil {
		configJSON = string(req.ConfigJSON)
	}

	id := uuid.New().String()
	_, err := h.DB.Exec(
		`INSERT INTO cluster_services (id, cluster_id, service_name, status, config_json)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (cluster_id, service_name) DO UPDATE SET status = $4, config_json = $5`,
		id, clusterID, serviceName, req.Status, configJSON)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update service: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"status": req.Status, "service": serviceName})
}

type validateRequest struct {
	Services []string `json:"services"`
}

// ValidateServices performs a dry-run dependency check.
func (h *ServiceHandler) ValidateServices(c echo.Context) error {
	var req validateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	cat := catalog.GetCatalog()
	if err := catalog.ValidateDependencies(cat, req.Services); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": err.Error(),
		})
	}

	order, err := catalog.ResolveDependencyOrder(cat, req.Services)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
		"order": order,
	})
}

func (h *ServiceHandler) getEnabledServices(clusterID string) ([]string, error) {
	rows, err := h.DB.Query(
		`SELECT service_name FROM cluster_services WHERE cluster_id = $1 AND status = 'enabled'`,
		clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		services = append(services, name)
	}
	return services, nil
}
