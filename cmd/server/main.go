package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/api"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/metrics"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/sciffer/agentbox/pkg/permissions"
	"github.com/sciffer/agentbox/pkg/proxy"
	"github.com/sciffer/agentbox/pkg/users"
	"github.com/sciffer/agentbox/pkg/validator"
)

var (
	configPath = flag.String("config", "config/config.yaml", "path to configuration file")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	log, err := logger.New(cfg.Server.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() {
		//nolint:errcheck // Best effort sync on shutdown, ignore error
		log.Sync()
	}()

	log.Info("starting agentbox server", zap.String("version", "1.0.0"))

	// Initialize database
	db, err := database.NewDB(log.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()
	log.Info("database initialized")

	// Initialize user service
	userService := users.NewService(db, log.Logger)

	// Ensure default admin user exists
	ctx := context.Background()
	if err := userService.EnsureDefaultAdmin(ctx); err != nil {
		log.Warn("failed to ensure default admin", zap.Error(err))
	}

	// Initialize auth service
	authService := auth.NewService(db, userService, log.Logger)

	// Initialize permission service
	permissionService := permissions.NewService(db, log.Logger)

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Verify Kubernetes connectivity
	if err := k8sClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("kubernetes health check failed: %w", err)
	}

	version, err := k8sClient.GetServerVersion(ctx)
	if err != nil {
		log.Warn("failed to get kubernetes version", zap.Error(err))
	} else {
		log.Info("connected to kubernetes", zap.String("version", version))
	}

	// Initialize validator
	val := validator.New(
		10000,              // max CPU: 10 cores
		10*1024*1024*1024,  // max Memory: 10Gi
		100*1024*1024*1024, // max Storage: 100Gi
		cfg.Timeouts.MaxTimeout,
	)

	// Initialize orchestrator
	orch := orchestrator.New(k8sClient, cfg, log)

	// Initialize WebSocket proxy
	var k8sInterface k8s.ClientInterface = k8sClient
	proxyHandler := proxy.NewProxy(k8sInterface, log)

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(db, orch, log.Logger)
	go metricsCollector.Start(ctx)
	defer metricsCollector.Stop()

	// Initialize all handlers
	handler := api.NewHandler(orch, val, log)
	authHandler := api.NewAuthHandler(authService, userService, log)
	userHandler := api.NewUserHandler(userService, authService, log)
	apiKeyHandler := api.NewAPIKeyHandler(authService, permissionService, log)
	metricsHandler := api.NewMetricsHandler(db, log)
	permissionHandler := api.NewPermissionHandler(permissionService, userService, log)

	// Create router with full configuration
	routerConfig := &api.RouterConfig{
		Handler:           handler,
		AuthHandler:       authHandler,
		UserHandler:       userHandler,
		APIKeyHandler:     apiKeyHandler,
		MetricsHandler:    metricsHandler,
		PermissionHandler: permissionHandler,
		ProxyHandler:      proxyHandler,
		AuthService:       authService,
	}
	router := api.NewRouter(routerConfig)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", zap.String("address", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info("shutting down server...")
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped")
	return nil
}
