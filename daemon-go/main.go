package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/oho/knowledge-refinery-daemon/internal/api"
	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/pipeline"
	"github.com/oho/knowledge-refinery-daemon/internal/server"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func main() {
	// Configure structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("Starting Knowledge Refinery daemon...")

	// Load config
	cfg := config.LoadConfig()
	slog.Info("Configuration loaded", "data_dir", cfg.DataDir, "port", cfg.Port)

	// Initialize database
	db, err := storage.NewDatabase(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Initialize(); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	slog.Info("Database initialized", "path", cfg.DBPath)

	// Initialize vector store (using same SQLite database)
	vs, err := storage.NewVectorStore(db.DB(), 768)
	if err != nil {
		slog.Error("Failed to initialize vector store", "error", err)
		os.Exit(1)
	}
	if err := vs.LoadAll(); err != nil {
		slog.Error("Failed to load vectors", "error", err)
		os.Exit(1)
	}
	slog.Info("Vector store loaded", "count", vs.Count())

	// Initialize LM Studio client
	lm := lmstudio.NewClient(cfg.LMStudio.BaseURL, cfg.LMStudio.Timeout)
	if lm.HealthCheck() {
		models := lm.ListModels()
		var ids []string
		for _, m := range models {
			ids = append(ids, m.ID)
		}
		slog.Info("LM Studio connected", "models", strings.Join(ids, ", "))
	} else {
		slog.Warn("LM Studio not available - embedding/annotation will fail")
	}

	// Initialize orchestrator
	orch := pipeline.NewOrchestrator(db, vs, lm, cfg)

	// Build HTTP router
	r := server.NewRouter()

	// Health endpoint
	r.Get("/health", server.HealthHandler(cfg, db, vs, lm))

	// Mount API routers matching Python's prefix structure
	r.Mount("/volumes", api.VolumesRouter(db))
	r.Mount("/ingest", api.IngestRouter(orch))
	r.Mount("/search", api.SearchRouter(lm, vs, db))
	r.Mount("/evidence", api.EvidenceRouter(db))
	r.Mount("/universe", api.UniverseRouter(db, vs))
	r.Mount("/concepts", api.ConceptsRouter(db, orch.Conceptualizer()))

	// Write PID file
	pidPath := filepath.Join(cfg.DataDir, "daemon.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
	defer os.Remove(pidPath)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("  Knowledge Refinery Daemon (Go)\n")
	fmt.Printf("  http://%s\n", addr)
	fmt.Printf("  Data dir: %s\n", cfg.DataDir)
	fmt.Printf("  LM Studio: %s\n", cfg.LMStudio.BaseURL)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	slog.Info("Daemon ready", "addr", addr)

	// Graceful shutdown on signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	slog.Info("Daemon stopped")
}
