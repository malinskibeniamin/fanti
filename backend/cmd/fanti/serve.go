package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/malinskibeniamin/fanti/backend/internal/config"
	"github.com/malinskibeniamin/fanti/backend/internal/db"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
)

type serveCmd struct{}

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 15 * time.Second
)

func (serveCmd) Run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		return fmt.Errorf("build handler: %w", err)
	}

	if cfg.StaticDir != "" {
		handler = withStaticApp(handler, cfg.StaticDir)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	go func() {
		logger.InfoContext(ctx, "server listening", slog.String("addr", cfg.ListenAddr))

		if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	select {
	case err = <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.InfoContext(shutdownCtx, "server stopped")

	return nil
}
