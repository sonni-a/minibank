package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/sonni-a/minibank/api/payment"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"github.com/sonni-a/minibank/payment-service/internal/service"
	"github.com/sonni-a/minibank/pkg/db"
	"github.com/sonni-a/minibank/pkg/env"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/migrate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dbConn := db.Connect()
	defer dbConn.Close()

	migrate.Run(dbConn, "file://payment-service/internal/db/migrations")

	userAddr := env.Getenv("USER_SERVICE_ADDR", "localhost:50052")
	userConn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to user-service", "addr", userAddr, "error", err)
		os.Exit(1)
	}
	defer userConn.Close()

	repo := repository.NewPaymentRepository(dbConn)
	userClient := user.NewUserServiceClient(userConn)
	paymentService := service.NewPaymentService(repo, userClient)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor()),
	)
	payment.RegisterPaymentServiceServer(grpcServer, paymentService)
	reflection.Register(grpcServer)

	slog.Info("payment service started", "addr", ":50053", "user-service", userAddr)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve failed", "error", err)
		os.Exit(1)
	}
}
