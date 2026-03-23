package main

import (
	"log"
	"net"

	"github.com/sonni-a/minibank/payment-service/internal/db"
	"github.com/sonni-a/minibank/payment-service/internal/grpc/payment"
	"github.com/sonni-a/minibank/payment-service/internal/middleware"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"github.com/sonni-a/minibank/payment-service/internal/service"
	"github.com/sonni-a/minibank/pkg/migrate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbConn := db.Connect("postgres://user:pass@postgres:5432/payment_db?sslmode=disable")

	migrate.Run(dbConn, "file://payment-service/internal/db/migrations")

	repo := repository.NewPaymentRepository(dbConn)

	paymentService := service.NewPaymentService(repo)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor()),
	)

	payment.RegisterPaymentServiceServer(grpcServer, paymentService)

	reflection.Register(grpcServer)

	log.Println("Payment Service running on :50053")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
