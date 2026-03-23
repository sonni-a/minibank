package main

import (
	"log"
	"net"

	"github.com/sonni-a/minibank/pkg/migrate"
	"github.com/sonni-a/minibank/user-service/internal/db"
	"github.com/sonni-a/minibank/user-service/internal/grpc/user"
	"github.com/sonni-a/minibank/user-service/internal/middleware"
	"github.com/sonni-a/minibank/user-service/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbConn := db.Connect("postgres://user:pass@postgres:5432/user_db?sslmode=disable")

	migrate.Run(dbConn, "file://user-service/internal/db/migrations")

	userService := service.NewUserService(dbConn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor()),
	)

	user.RegisterUserServiceServer(grpcServer, userService)

	reflection.Register(grpcServer)

	log.Println("User Service running on :50052")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
