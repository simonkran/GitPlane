package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/simonkran/gitplane/api/middleware"
)

// GitHandler groups git connection endpoints.
type GitHandler struct {
	DB *sql.DB
}

// NewGitHandler creates a new GitHandler.
func NewGitHandler(db *sql.DB) *GitHandler {
	return &GitHandler{DB: db}
}

type connectGitRequest struct {
	Provider      string `json:"provider"`
	RepoURL       string `json:"repoUrl"`
	AccessToken   string `json:"accessToken"`
	DefaultBranch string `json:"defaultBranch"`
}

type gitConnectionResponse struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"orgId"`
	Provider      string    `json:"provider"`
	RepoURL       string    `json:"repoUrl"`
	DefaultBranch string    `json:"defaultBranch"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Connect stores a new git connection.
func (h *GitHandler) Connect(c echo.Context) error {
	claims := middleware.GetClaims(c)

	var req connectGitRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Provider == "" || req.RepoURL == "" || req.AccessToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider, repoUrl, and accessToken are required")
	}

	if req.Provider != "github" && req.Provider != "gitlab" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider must be 'github' or 'gitlab'")
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := h.DB.Exec(
		`INSERT INTO git_connections (id, org_id, provider, access_token, repo_url, default_branch, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, claims.OrgID, req.Provider, req.AccessToken, req.RepoURL, req.DefaultBranch, now)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create git connection")
	}

	return c.JSON(http.StatusCreated, gitConnectionResponse{
		ID:            id,
		OrgID:         claims.OrgID,
		Provider:      req.Provider,
		RepoURL:       req.RepoURL,
		DefaultBranch: req.DefaultBranch,
		CreatedAt:     now,
	})
}

// Status returns the current git connection for the org.
func (h *GitHandler) Status(c echo.Context) error {
	claims := middleware.GetClaims(c)

	rows, err := h.DB.Query(
		`SELECT id, org_id, provider, repo_url, default_branch, created_at
		 FROM git_connections WHERE org_id = $1 ORDER BY created_at DESC`,
		claims.OrgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer rows.Close()

	var connections []gitConnectionResponse
	for rows.Next() {
		var conn gitConnectionResponse
		if err := rows.Scan(&conn.ID, &conn.OrgID, &conn.Provider, &conn.RepoURL, &conn.DefaultBranch, &conn.CreatedAt); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "scan error")
		}
		connections = append(connections, conn)
	}

	if connections == nil {
		connections = []gitConnectionResponse{}
	}
	return c.JSON(http.StatusOK, connections)
}

// Disconnect removes a git connection.
func (h *GitHandler) Disconnect(c echo.Context) error {
	claims := middleware.GetClaims(c)

	result, err := h.DB.Exec(`DELETE FROM git_connections WHERE org_id = $1`, claims.OrgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no git connection found")
	}

	return c.NoContent(http.StatusNoContent)
}
