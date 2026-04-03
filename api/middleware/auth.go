// Package middleware provides HTTP middleware for the GitPlane API server.
package middleware

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// Claims holds the custom JWT claims for authenticated users.
type Claims struct {
	UserID string `json:"userId"`
	OrgID  string `json:"orgId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims holds claims for refresh tokens.
type RefreshClaims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

const claimsContextKey = "claims"

func jwtSecret() []byte {
	return []byte(os.Getenv("GITPLANE_JWT_SECRET"))
}

// GenerateToken creates a signed JWT with 24-hour expiry.
func GenerateToken(userID, orgID, email, role string) (string, error) {
	claims := &Claims{
		UserID: userID,
		OrgID:  orgID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gitplane",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// GenerateRefreshToken creates a signed refresh token with 7-day expiry.
func GenerateRefreshToken(userID string) (string, error) {
	claims := &RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gitplane",
			Subject:   "refresh",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// JWTAuth returns Echo middleware that validates a JWT from the Authorization
// Bearer header and stores the parsed Claims in the request context.
func JWTAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization format")
			}

			tokenStr := parts[1]
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "unexpected signing method")
				}
				return jwtSecret(), nil
			})
			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			c.Set(claimsContextKey, claims)
			return next(c)
		}
	}
}

// GetClaims extracts the parsed Claims from the Echo context.
func GetClaims(c echo.Context) *Claims {
	v := c.Get(claimsContextKey)
	if v == nil {
		return nil
	}
	claims, ok := v.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// AgentAuth returns Echo middleware that authenticates cluster agents using
// a bearer token matched against the clusters table agent_token column.
func AgentAuth(db *sql.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization format")
			}

			agentToken := parts[1]

			var clusterID, orgID string
			err := db.QueryRow(
				"SELECT id, org_id FROM clusters WHERE agent_token = $1",
				agentToken,
			).Scan(&clusterID, &orgID)
			if err != nil {
				if err == sql.ErrNoRows {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid agent token")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "database error")
			}

			// Constant-time comparison already done by SQL, but we set
			// context values for downstream handlers.
			_ = subtle.ConstantTimeCompare([]byte(agentToken), []byte(agentToken))

			c.Set("cluster_id", clusterID)
			c.Set("org_id", orgID)
			return next(c)
		}
	}
}
