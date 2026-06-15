package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/pkg/db"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/migrate"
	"github.com/sonni-a/minibank/user-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dbConn := db.Connect()
	defer dbConn.Close()

	migrate.Run(dbConn, "file://user-service/internal/db/migrations")

	userService := service.NewUserService(dbConn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor(
			"/user.UserService/CreateUser",
		)),
	)
	user.RegisterUserServiceServer(grpcServer, userService)
	reflection.Register(grpcServer)

	slog.Info("user service started", "addr", ":50052")
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve failed", "error", err)
		os.Exit(1)
	}
}
