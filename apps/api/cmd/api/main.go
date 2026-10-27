package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"compytition/internal/auth"
	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.AutoMigrate {
		if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
			log.Error("migrations failed", "error", err)
			os.Exit(1)
		}
	}

	store, err := db.NewStore(ctx, cfg.DatabaseURL, 20)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	authSvc := auth.NewService(store, cfg, log)
	router := server.NewRouter(&server.API{Store: store, Cfg: cfg, Log: log, Auth: authSvc})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := store.DeleteExpiredSessions(cleanupCtx); err != nil {
					log.Warn("session cleanup failed", "error", err)
				}
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		log.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		opts.Level = slog.LevelDebug
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	}
	if cfg.Env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
