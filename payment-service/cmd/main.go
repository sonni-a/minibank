package main

import (
	"log"
	"net"
	"os"
	"strings"

	"github.com/sonni-a/minibank/api/payment"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"github.com/sonni-a/minibank/payment-service/internal/service"
	"github.com/sonni-a/minibank/pkg/db"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/migrate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbConn := db.Connect()
	defer dbConn.Close()

	migrate.Run(dbConn, "file://payment-service/internal/db/migrations")

	userAddr := getenv("USER_SERVICE_ADDR", "localhost:50052")
	userConn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("user-service dial: %v", err)
	}
	defer userConn.Close()

	repo := repository.NewPaymentRepository(dbConn)
	userClient := user.NewUserServiceClient(userConn)
	paymentService := service.NewPaymentService(repo, userClient)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor()),
	)
	payment.RegisterPaymentServiceServer(grpcServer, paymentService)
	reflection.Register(grpcServer)

	log.Printf("Payment Service on :50053 (user-service=%s)", userAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
