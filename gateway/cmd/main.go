package main

import (
	"log"
	"net/http"

	"github.com/sonni-a/minibank/gateway/internal/httpapi"
	"github.com/sonni-a/minibank/pkg/env"
)

func main() {
	authAddr := env.Getenv("AUTH_SERVICE_ADDR", "localhost:50051")
	userAddr := env.Getenv("USER_SERVICE_ADDR", "localhost:50052")
	payAddr := env.Getenv("PAYMENT_SERVICE_ADDR", "localhost:50053")

	srv, err := httpapi.New(authAddr, userAddr, payAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.Println("gateway close:", err)
		}
	}()

	addr := httpapi.ListenAddr()
	log.Printf("gateway listening on %s (auth=%s user=%s payment=%s)", addr, authAddr, userAddr, payAddr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

