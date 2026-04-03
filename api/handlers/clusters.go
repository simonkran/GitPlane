package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/simonkran/gitplane/api/middleware"
)

// ClusterHandler groups cluster-related endpoints.
type ClusterHandler struct {
	DB *sql.DB
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(db *sql.DB) *ClusterHandler {
	return &ClusterHandler{DB: db}
}

type createClusterRequest struct {
	Name        string `json:"name"`
	Stage       string `json:"stage"`
	Type        string `json:"type"`
	DNSName     string `json:"dnsName"`
	ClusterSize string `json:"clusterSize"`
	GitConnID   string `json:"gitConnId"`
	GitPath     string `json:"gitPath"`
}

type updateClusterRequest struct {
	Name        string          `json:"name"`
	Stage       string          `json:"stage"`
	DNSName     string          `json:"dnsName"`
	ClusterSize string          `json:"clusterSize"`
	GitConnID   string          `json:"gitConnId"`
	GitPath     string          `json:"gitPath"`
	ConfigJSON  json.RawMessage `json:"configJson"`
}

type clusterResponse struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"orgId"`
	GitConnID   *string         `json:"gitConnId,omitempty"`
	Name        string          `json:"name"`
	Stage       string          `json:"stage"`
	Type        string          `json:"type"`
	DNSName     string          `json:"dnsName"`
	ClusterSize string          `json:"clusterSize"`
	ConfigJSON  json.RawMessage `json:"configJson"`
	AgentToken  *string         `json:"agentToken,omitempty"`
	GitPath     string          `json:"gitPath"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Status      *statusResponse `json:"status,omitempty"`
}

type statusResponse struct {
	LastSeenAt             *time.Time      `json:"lastSeenAt,omitempty"`
	SyncReady              *bool           `json:"syncReady,omitempty"`
	SyncRevision           *string         `json:"syncRevision,omitempty"`
	ComponentsOK           *int            `json:"componentsOk,omitempty"`
	ComponentsTotal        *int            `json:"componentsTotal,omitempty"`
	HelmreleasesRunning    *int            `json:"helmreleasesRunning,omitempty"`
	HelmreleasesFailing    *int            `json:"helmreleasesFailing,omitempty"`
	KustomizationsRunning  *int            `json:"kustomizationsRunning,omitempty"`
	KustomizationsFailing  *int            `json:"kustomizationsFailing,omitempty"`
	FluxReport             json.RawMessage `json:"fluxReport,omitempty"`
}

func generateAgentToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// List returns all clusters for the authenticated user's organization.
func (h *ClusterHandler) List(c echo.Context) error {
	claims := middleware.GetClaims(c)

	rows, err := h.DB.Query(
		`SELECT id, org_id, git_conn_id, name, stage, type, dns_name, cluster_size, config_json, agent_token, git_path, created_at, updated_at
		 FROM clusters WHERE org_id = $1 ORDER BY created_at DESC`, claims.OrgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer rows.Close()

	var clusters []clusterResponse
	for rows.Next() {
		var cl clusterResponse
		var configRaw []byte
		if err := rows.Scan(&cl.ID, &cl.OrgID, &cl.GitConnID, &cl.Name, &cl.Stage, &cl.Type, &cl.DNSName, &cl.ClusterSize, &configRaw, &cl.AgentToken, &cl.GitPath, &cl.CreatedAt, &cl.UpdatedAt); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "scan error")
		}
		cl.ConfigJSON = json.RawMessage(configRaw)
		clusters = append(clusters, cl)
	}

	if clusters == nil {
		clusters = []clusterResponse{}
	}
	return c.JSON(http.StatusOK, clusters)
}

// Create creates a new cluster.
func (h *ClusterHandler) Create(c echo.Context) error {
	claims := middleware.GetClaims(c)

	var req createClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" || req.Stage == "" || req.Type == "" || req.GitPath == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, stage, type, and gitPath are required")
	}

	if req.ClusterSize == "" {
		req.ClusterSize = "medium"
	}

	agentToken, err := generateAgentToken()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate agent token")
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	var gitConnID *string
	if req.GitConnID != "" {
		gitConnID = &req.GitConnID
	}

	_, err = h.DB.Exec(
		`INSERT INTO clusters (id, org_id, git_conn_id, name, stage, type, dns_name, cluster_size, config_json, agent_token, git_path, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		id, claims.OrgID, gitConnID, req.Name, req.Stage, req.Type, req.DNSName, req.ClusterSize, "{}", agentToken, req.GitPath, now, now)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create cluster: "+err.Error())
	}

	return c.JSON(http.StatusCreated, clusterResponse{
		ID:          id,
		OrgID:       claims.OrgID,
		GitConnID:   gitConnID,
		Name:        req.Name,
		Stage:       req.Stage,
		Type:        req.Type,
		DNSName:     req.DNSName,
		ClusterSize: req.ClusterSize,
		ConfigJSON:  json.RawMessage("{}"),
		AgentToken:  &agentToken,
		GitPath:     req.GitPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

// Get returns a single cluster with its latest status.
func (h *ClusterHandler) Get(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	var cl clusterResponse
	var configRaw []byte
	err := h.DB.QueryRow(
		`SELECT id, org_id, git_conn_id, name, stage, type, dns_name, cluster_size, config_json, agent_token, git_path, created_at, updated_at
		 FROM clusters WHERE id = $1 AND org_id = $2`, clusterID, claims.OrgID,
	).Scan(&cl.ID, &cl.OrgID, &cl.GitConnID, &cl.Name, &cl.Stage, &cl.Type, &cl.DNSName, &cl.ClusterSize, &configRaw, &cl.AgentToken, &cl.GitPath, &cl.CreatedAt, &cl.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	cl.ConfigJSON = json.RawMessage(configRaw)

	// Fetch latest status.
	var st statusResponse
	var fluxRaw []byte
	err = h.DB.QueryRow(
		`SELECT last_seen_at, sync_ready, sync_revision, components_ok, components_total,
		        helmreleases_running, helmreleases_failing, kustomizations_running, kustomizations_failing, flux_report
		 FROM cluster_status WHERE cluster_id = $1 ORDER BY updated_at DESC LIMIT 1`, clusterID,
	).Scan(&st.LastSeenAt, &st.SyncReady, &st.SyncRevision, &st.ComponentsOK, &st.ComponentsTotal,
		&st.HelmreleasesRunning, &st.HelmreleasesFailing, &st.KustomizationsRunning, &st.KustomizationsFailing, &fluxRaw)
	if err == nil {
		st.FluxReport = json.RawMessage(fluxRaw)
		cl.Status = &st
	}

	return c.JSON(http.StatusOK, cl)
}

// Update updates a cluster's configuration.
func (h *ClusterHandler) Update(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	var req updateClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	now := time.Now().UTC()
	configJSON := "{}"
	if req.ConfigJSON != nil {
		configJSON = string(req.ConfigJSON)
	}

	var gitConnID *string
	if req.GitConnID != "" {
		gitConnID = &req.GitConnID
	}

	result, err := h.DB.Exec(
		`UPDATE clusters SET name = $1, stage = $2, dns_name = $3, cluster_size = $4, git_conn_id = $5, git_path = $6, config_json = $7, updated_at = $8
		 WHERE id = $9 AND org_id = $10`,
		req.Name, req.Stage, req.DNSName, req.ClusterSize, gitConnID, req.GitPath, configJSON, now, clusterID, claims.OrgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// Delete removes a cluster.
func (h *ClusterHandler) Delete(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	result, err := h.DB.Exec(`DELETE FROM clusters WHERE id = $1 AND org_id = $2`, clusterID, claims.OrgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	return c.NoContent(http.StatusNoContent)
}

// AgentInstall returns the YAML manifest to install the agent in a cluster.
func (h *ClusterHandler) AgentInstall(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	var agentToken, clusterName string
	err := h.DB.QueryRow(
		`SELECT agent_token, name FROM clusters WHERE id = $1 AND org_id = $2`,
		clusterID, claims.OrgID,
	).Scan(&agentToken, &clusterName)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	apiURL := c.Request().Header.Get("X-GitPlane-API-URL")
	if apiURL == "" {
		apiURL = "https://api.gitplane.io"
	}

	manifest := generateAgentManifest(clusterName, agentToken, apiURL)
	return c.String(http.StatusOK, manifest)
}

func generateAgentManifest(clusterName, agentToken, apiURL string) string {
	return fmt.Sprintf(`---
apiVersion: v1
kind: Namespace
metadata:
  name: gitplane-agent
---
apiVersion: v1
kind: Secret
metadata:
  name: gitplane-agent
  namespace: gitplane-agent
type: Opaque
stringData:
  GITPLANE_API_URL: "%s"
  GITPLANE_AGENT_TOKEN: "%s"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitplane-agent
  namespace: gitplane-agent
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gitplane-agent
rules:
  - apiGroups: ["fluxcd.controlplane.io"]
    resources: ["fluxreports"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gitplane-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gitplane-agent
subjects:
  - kind: ServiceAccount
    name: gitplane-agent
    namespace: gitplane-agent
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitplane-agent
  namespace: gitplane-agent
  labels:
    app: gitplane-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitplane-agent
  template:
    metadata:
      labels:
        app: gitplane-agent
    spec:
      serviceAccountName: gitplane-agent
      containers:
        - name: agent
          image: ghcr.io/simonkran/gitplane-agent:latest
          envFrom:
            - secretRef:
                name: gitplane-agent
          env:
            - name: GITPLANE_REPORT_INTERVAL
              value: "60s"
          resources:
            limits:
              memory: 64Mi
              cpu: 50m
            requests:
              memory: 32Mi
              cpu: 10m
`, apiURL, agentToken)
}

// Status returns the latest FluxReport data for a cluster.
func (h *ClusterHandler) Status(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	// Verify cluster belongs to org.
	var exists bool
	err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1 AND org_id = $2)`, clusterID, claims.OrgID).Scan(&exists)
	if err != nil || !exists {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	var st statusResponse
	var fluxRaw []byte
	err = h.DB.QueryRow(
		`SELECT last_seen_at, sync_ready, sync_revision, components_ok, components_total,
		        helmreleases_running, helmreleases_failing, kustomizations_running, kustomizations_failing, flux_report
		 FROM cluster_status WHERE cluster_id = $1 ORDER BY updated_at DESC LIMIT 1`, clusterID,
	).Scan(&st.LastSeenAt, &st.SyncReady, &st.SyncRevision, &st.ComponentsOK, &st.ComponentsTotal,
		&st.HelmreleasesRunning, &st.HelmreleasesFailing, &st.KustomizationsRunning, &st.KustomizationsFailing, &fluxRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, map[string]string{"status": "no data"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	st.FluxReport = json.RawMessage(fluxRaw)

	return c.JSON(http.StatusOK, st)
}

// History returns the generation history for a cluster.
func (h *ClusterHandler) History(c echo.Context) error {
	claims := middleware.GetClaims(c)
	clusterID := c.Param("id")

	// Verify cluster belongs to org.
	var exists bool
	err := h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM clusters WHERE id = $1 AND org_id = $2)`, clusterID, claims.OrgID).Scan(&exists)
	if err != nil || !exists {
		return echo.NewHTTPError(http.StatusNotFound, "cluster not found")
	}

	rows, err := h.DB.Query(
		`SELECT id, cluster_id, triggered_by, git_commit_sha, status, error_message, created_at
		 FROM generation_history WHERE cluster_id = $1 ORDER BY created_at DESC LIMIT 50`, clusterID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer rows.Close()

	type historyEntry struct {
		ID           string     `json:"id"`
		ClusterID    string     `json:"clusterId"`
		TriggeredBy  *string    `json:"triggeredBy,omitempty"`
		GitCommitSHA *string    `json:"gitCommitSha,omitempty"`
		Status       string     `json:"status"`
		ErrorMessage *string    `json:"errorMessage,omitempty"`
		CreatedAt    time.Time  `json:"createdAt"`
	}

	var history []historyEntry
	for rows.Next() {
		var h historyEntry
		if err := rows.Scan(&h.ID, &h.ClusterID, &h.TriggeredBy, &h.GitCommitSHA, &h.Status, &h.ErrorMessage, &h.CreatedAt); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "scan error")
		}
		history = append(history, h)
	}

	if history == nil {
		history = []historyEntry{}
	}
	return c.JSON(http.StatusOK, history)
}
