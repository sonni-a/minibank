package main

import (
	"log"
	"net"

	"github.com/sonni-a/minibank/auth-service/internal/db"
	"github.com/sonni-a/minibank/auth-service/internal/grpc/auth"
	"github.com/sonni-a/minibank/auth-service/internal/service"
	"github.com/sonni-a/minibank/pkg/migrate"
	pkgredis "github.com/sonni-a/minibank/pkg/redis"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbConn := db.Connect("postgres://user:pass@postgres:5432/auth_db?sslmode=disable")

	migrate.Run(dbConn, "file://auth-service/internal/db/migrations")

	rdb := pkgredis.ConnectRedis("redis:6379")
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Println("Error closing Redis:", err)
		}
	}()

	authService := service.NewAuthService(dbConn)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	auth.RegisterAuthServiceServer(grpcServer, authService)

	reflection.Register(grpcServer)

	log.Println("Auth Service running on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
