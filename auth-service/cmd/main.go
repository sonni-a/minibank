package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/sonni-a/minibank/api/auth"
	"github.com/sonni-a/minibank/auth-service/internal/service"
	"github.com/sonni-a/minibank/pkg/db"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/migrate"
	pkgredis "github.com/sonni-a/minibank/pkg/redis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dbConn := db.Connect()
	defer dbConn.Close()

	migrate.Run(dbConn, "file://auth-service/internal/db/migrations")

	rdb := pkgredis.Connect(os.Getenv("REDIS_ADDR"))
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("error closing Redis", "error", err)
		}
	}()

	authService := service.NewAuthService(dbConn, rdb)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor(
			"/auth.AuthService/Register",
			"/auth.AuthService/Login",
			"/auth.AuthService/RefreshToken",
		)),
	)
	auth.RegisterAuthServiceServer(grpcServer, authService)
	reflection.Register(grpcServer)

	slog.Info("auth service started", "addr", ":50051")
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve failed", "error", err)
		os.Exit(1)
	}
}
