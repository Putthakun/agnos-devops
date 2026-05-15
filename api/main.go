package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// ── Structured JSON logger (Req 5: Observability) ────────────────────────────

func setupLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

// ── Middleware ────────────────────────────────────────────────────────────────

// JSONLoggerMiddleware logs every request as structured JSON
func JSONLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next() // process request

		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"env", os.Getenv("APP_ENV"),
		)
	}
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func healthHandler(c *gin.Context) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "api",
		"env":       env,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

// ── Router ────────────────────────────────────────────────────────────────────

func setupRouter() *gin.Engine {
	env := os.Getenv("APP_ENV")

	// Use release mode in non-dev environments (less noisy gin internal logs)
	if env == "prod" || env == "uat" {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New() instead of gin.Default() — we control every middleware ourselves
	r := gin.New()
	r.Use(gin.Recovery())        // auto-recover from panics → 500 instead of crash
	r.Use(JSONLoggerMiddleware()) // structured JSON logs for every request

	r.GET("/health", healthHandler)

	return r
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	setupLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := setupRouter()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown — K8s sends SIGTERM before killing the pod
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("api server starting", "port", port, "env", os.Getenv("APP_ENV"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("api server shutting down gracefully")
}