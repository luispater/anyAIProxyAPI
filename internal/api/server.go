package api

import (
	"context"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/proxy"
	"github.com/playwright-community/playwright-go"
)

// Server represents the API server
type Server struct {
	engine    *gin.Engine
	server    *http.Server
	queue     *RequestQueue
	processor *ChatProcessor
	handlers  *APIHandlers
}

// ServerConfig contains configuration for the API server
type ServerConfig struct {
	Port   string
	Debug  bool
	Pages  map[string]playwright.Page
	Proxys map[string]*proxy.Proxy
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig, appConfig *config.AppConfig) *Server {
	// Set gin mode
	if !config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create processor
	processor := NewChatProcessor(appConfig, config.Pages, config.Proxys, config.Debug)

	// Create queue
	queue := NewRequestQueue(processor)

	// Create handlers
	handlers := NewAPIHandlers(appConfig, queue, config.Pages, config.Debug)

	// Create gin engine
	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	// Create server instance
	s := &Server{
		engine:    engine,
		queue:     queue,
		processor: processor,
		handlers:  handlers,
	}

	// Setup routes
	s.setupRoutes()

	// Create HTTP server
	s.server = &http.Server{
		Addr:    ":" + config.Port,
		Handler: engine,
	}

	return s
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// OpenAI compatible API routes
	v1 := s.engine.Group("/v1")
	{
		v1.POST("/chat/completions", s.handlers.ChatCompletions)
	}

	// Root endpoint
	s.engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Any AI Proxy API Server",
			"version": "1.0.0",
			"endpoints": []string{
				"POST /v1/chat/completions",
			},
		})
	})
}

// Start starts the API server
func (s *Server) Start() error {
	// Start the request queue
	if err := s.queue.Start(); err != nil {
		return fmt.Errorf("failed to start request queue: %v", err)
	}

	log.Debugf("Starting API server on %s", s.server.Addr)

	// Start the HTTP server
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %v", err)
	}

	return nil
}

// Stop gracefully stops the API server
func (s *Server) Stop(ctx context.Context) error {
	log.Debug("Stopping API server...")

	// Stop the request queue
	if err := s.queue.Stop(); err != nil {
		log.Debugf("Error stopping request queue: %v", err)
	}

	// Shutdown the HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %v", err)
	}

	log.Debug("API server stopped")
	return nil
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
