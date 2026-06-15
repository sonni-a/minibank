package main

import (
	"log/slog"
	"net/http"
	"os"

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
	slog.Info("gateway started", "addr", addr, "auth", authAddr, "user", userAddr, "payment", payAddr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
