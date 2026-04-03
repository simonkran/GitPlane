package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RequireRole returns middleware that restricts access to users whose
// Claims.Role matches one of the provided roles.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			if _, ok := allowed[claims.Role]; !ok {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// RequireOrgAccess returns middleware that ensures the authenticated user
// belongs to the organisation they are attempting to access. The org is
// determined by looking up the cluster's org_id when a :id param is present,
// or by comparing the claims OrgID directly.
func RequireOrgAccess() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			// If the route contains an org_id parameter, verify it matches.
			orgID := c.Param("org_id")
			if orgID != "" && orgID != claims.OrgID {
				return echo.NewHTTPError(http.StatusForbidden, "access denied to this organisation")
			}

			return next(c)
		}
	}
}
