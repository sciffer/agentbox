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

	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/api"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/sciffer/agentbox/pkg/proxy"
	"github.com/sciffer/agentbox/pkg/validator"
	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "config/config.yaml", "path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.Server.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("starting agentbox server", zap.String("version", "1.0.0"))

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		log.Fatal("failed to create kubernetes client", zap.Error(err))
	}

	// Verify Kubernetes connectivity
	ctx := context.Background()
	if err := k8sClient.HealthCheck(ctx); err != nil {
		log.Fatal("kubernetes health check failed", zap.Error(err))
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

	// Initialize API handler
	handler := api.NewHandler(orch, val, log)

	// Initialize WebSocket proxy
	// Use ClientInterface for better testability
	var k8sInterface k8s.ClientInterface = k8sClient
	proxyHandler := proxy.NewProxy(k8sInterface, log)

	// Create router
	router := api.NewRouter(handler, proxyHandler)

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
	go func() {
		log.Info("server listening", zap.String("address", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}
