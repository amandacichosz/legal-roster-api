package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"legal-bench-api/internal/config"
	"legal-bench-api/internal/database"
	"legal-bench-api/internal/db"
	"legal-bench-api/internal/email"
	"legal-bench-api/internal/httpserver"
	"legal-bench-api/internal/objectstore"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_ = godotenv.Load()

	cfg := config.FromEnv()
	ctx := context.Background()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)
	if err := bootstrapPlatformAdmin(ctx, cfg, queries); err != nil {
		logger.Error("bootstrap platform admin", "error", err)
		os.Exit(1)
	}

	resumeStore, err := objectstore.New(cfg)
	if err != nil {
		logger.Error("configure object storage", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpserver.New(pool, cfg.StaticDir, cfg.CORSOrigins, cfg.FrontendURL, cfg.SessionCookieSecure, email.NewMailer(cfg), resumeStore, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", server.Addr, "env", cfg.AppEnv, "static_dir", cfg.StaticDir)
		errCh <- server.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-stopCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
			os.Exit(1)
		}
	}
}
