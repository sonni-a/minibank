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

	"github.com/sonni-a/minibank/gateway/internal/httpapi"
	"github.com/sonni-a/minibank/pkg/env"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	authAddr := env.Getenv("AUTH_SERVICE_ADDR", "localhost:50051")
	userAddr := env.Getenv("USER_SERVICE_ADDR", "localhost:50052")
	payAddr := env.Getenv("PAYMENT_SERVICE_ADDR", "localhost:50053")

	srv, err := httpapi.New(authAddr, userAddr, payAddr)
	if err != nil {
		slog.Error("failed to init gateway", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			slog.Warn("error closing gateway", "error", err)
		}
	}()

	addr := httpapi.ListenAddr()
	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("gateway started", "addr", addr, "auth", authAddr, "user", userAddr, "payment", payAddr)
	<-quit
	slog.Info("shutting down gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
