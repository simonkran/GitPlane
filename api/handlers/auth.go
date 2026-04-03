// Package handlers implements the HTTP request handlers for the GitPlane API.
package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/simonkran/gitplane/api/middleware"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler groups authentication-related endpoints.
type AuthHandler struct {
	DB *sql.DB
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

type registerRequest struct {
	OrgName  string `json:"orgName"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type authResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken,omitempty"`
	UserID       string `json:"userId"`
	OrgID        string `json:"orgId"`
}

// Register creates a new organisation and admin user.
func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.OrgName == "" || req.Email == "" || req.Password == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "orgName, email, password, and name are required")
	}

	if len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, "password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
	}

	orgID := uuid.New().String()
	userID := uuid.New().String()
	now := time.Now().UTC()
	role := "admin"

	tx, err := h.DB.Begin()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}
	defer tx.Rollback() //nolint:errcheck

	slug := strings.ToLower(strings.ReplaceAll(req.OrgName, " ", "-"))
	_, err = tx.Exec(
		`INSERT INTO organizations (id, name, slug, created_at) VALUES ($1, $2, $3, $4)`,
		orgID, req.OrgName, slug, now,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, "organisation could not be created")
	}

	_, err = tx.Exec(
		`INSERT INTO users (id, org_id, email, password_hash, name, role, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, orgID, req.Email, string(hash), req.Name, role, now,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, "user could not be created (email may already exist)")
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit transaction")
	}

	token, err := middleware.GenerateToken(userID, orgID, req.Email, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	refreshToken, err := middleware.GenerateRefreshToken(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate refresh token")
	}

	return c.JSON(http.StatusCreated, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		UserID:       userID,
		OrgID:        orgID,
	})
}

// Login authenticates a user with email and password.
func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Email == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email and password are required")
	}

	var userID, orgID, passwordHash, role string
	err := h.DB.QueryRow(
		`SELECT id, org_id, password_hash, role FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &orgID, &passwordHash, &role)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}

	token, err := middleware.GenerateToken(userID, orgID, req.Email, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	refreshToken, err := middleware.GenerateRefreshToken(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate refresh token")
	}

	return c.JSON(http.StatusOK, authResponse{
		Token:        token,
		RefreshToken: refreshToken,
		UserID:       userID,
		OrgID:        orgID,
	})
}

// Refresh exchanges a valid refresh token for a new JWT.
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refreshToken is required")
	}

	claims := &middleware.RefreshClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("GITPLANE_JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired refresh token")
	}

	if claims.Subject != "refresh" {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid token type")
	}

	var orgID, email, role string
	err = h.DB.QueryRow(
		`SELECT org_id, email, role FROM users WHERE id = $1`,
		claims.UserID,
	).Scan(&orgID, &email, &role)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "database error")
	}

	newToken, err := middleware.GenerateToken(claims.UserID, orgID, email, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	return c.JSON(http.StatusOK, authResponse{
		Token:  newToken,
		UserID: claims.UserID,
		OrgID:  orgID,
	})
}
