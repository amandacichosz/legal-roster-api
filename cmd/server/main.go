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

	"legal-roster-api/internal/config"
	"legal-roster-api/internal/database"
	"legal-roster-api/internal/email"
	"legal-roster-api/internal/httpserver"
	"legal-roster-api/internal/objectstore"
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

	var databaseVersion string
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&databaseVersion); err != nil {
		logger.Error("query database version", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database", "version", databaseVersion)

	resumeStore, err := objectstore.New(cfg)
	if err != nil {
		logger.Error("configure object storage", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpserver.New(pool, cfg.StaticDir, cfg.CORSOrigins, cfg.FrontendURL, cfg.SessionCookieSecure, email.NewMailer(cfg), resumeStore, cfg.SupabaseURL, cfg.SupabasePublishableKey, logger),
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
