// Package api provides the GitPlane HTTP API server.
package api

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/simonkran/gitplane/api/handlers"
	"github.com/simonkran/gitplane/api/middleware"
	"github.com/simonkran/gitplane/api/ws"
)

// Server holds the Echo instance and dependencies.
type Server struct {
	Echo *echo.Echo
	DB   *sql.DB
}

// NewServer creates and configures the API server with all routes.
func NewServer(db *sql.DB) *Server {
	e := echo.New()
	e.HideBanner = true

	// Global middleware.
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	// Health check.
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Initialize handlers.
	authHandler := handlers.NewAuthHandler(db)
	clusterHandler := handlers.NewClusterHandler(db)
	serviceHandler := handlers.NewServiceHandler(db)
	generationHandler := handlers.NewGenerationHandler(db)
	gitHandler := handlers.NewGitHandler(db)
	agentHandler := handlers.NewAgentHandler(db)
	hub := ws.NewHub()
	go hub.Run()

	// API v1 routes.
	v1 := e.Group("/api/v1")

	// Public auth endpoints.
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)

	// Agent endpoint (agent token auth).
	agent := v1.Group("/agent")
	agent.Use(middleware.AgentAuth(db))
	agent.POST("/report", agentHandler.Report)

	// All other endpoints require JWT auth.
	authed := v1.Group("")
	authed.Use(middleware.JWTAuth())

	// Git connections.
	authed.POST("/git/connect", gitHandler.Connect)
	authed.GET("/git/status", gitHandler.Status)
	authed.DELETE("/git/disconnect", gitHandler.Disconnect)

	// Clusters.
	authed.GET("/clusters", clusterHandler.List)
	authed.POST("/clusters", clusterHandler.Create)
	authed.GET("/clusters/:id", clusterHandler.Get)
	authed.PUT("/clusters/:id", clusterHandler.Update)
	authed.DELETE("/clusters/:id", clusterHandler.Delete)
	authed.GET("/clusters/:id/agent-install", clusterHandler.AgentInstall)
	authed.GET("/clusters/:id/status", clusterHandler.Status)
	authed.GET("/clusters/:id/history", clusterHandler.History)

	// Service catalog.
	authed.GET("/catalog", serviceHandler.Catalog)
	authed.GET("/clusters/:id/services", serviceHandler.ListClusterServices)
	authed.PUT("/clusters/:id/services/:name", serviceHandler.UpdateClusterService)
	authed.POST("/clusters/:id/services/validate", serviceHandler.ValidateServices)

	// Generation.
	authed.POST("/clusters/:id/generate", generationHandler.Generate)
	authed.POST("/clusters/:id/generate/preview", generationHandler.Preview)
	authed.GET("/clusters/:id/generate/:gen_id", generationHandler.GetGeneration)

	// WebSocket for real-time cluster status.
	e.GET("/api/v1/ws/clusters/:id/status", ws.HandleWebSocket(hub))

	return &Server{Echo: e, DB: db}
}

// Start begins listening on the given address.
func (s *Server) Start(addr string) error {
	return s.Echo.Start(addr)
}
