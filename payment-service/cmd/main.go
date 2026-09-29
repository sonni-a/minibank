package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/sonni-a/minibank/api/payment"
	"github.com/sonni-a/minibank/api/user"
	paydb "github.com/sonni-a/minibank/payment-service/internal/db"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"github.com/sonni-a/minibank/payment-service/internal/service"
	"github.com/sonni-a/minibank/pkg/db"
	"github.com/sonni-a/minibank/pkg/env"
	"github.com/sonni-a/minibank/pkg/grpcclient"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/migrate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dbConn := db.Connect()
	defer dbConn.Close()

	migrate.Run(dbConn, paydb.FS)

	userAddr := env.Getenv("USER_SERVICE_ADDR", "localhost:50052")
	slog.Info("connecting to user-service", "addr", userAddr)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeout)
	defer dialCancel()
	userConn, err := grpcclient.Dial(dialCtx, userAddr)
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("grpc serve failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("payment service started", "addr", ":50053", "user-service", userAddr)
	<-quit
	slog.Info("shutting down payment service...")
	grpcServer.GracefulStop()
}
