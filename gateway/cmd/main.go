package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/sonni-a/minibank/gateway/internal/httpapi"
)

func main() {
	authAddr := getenv("AUTH_SERVICE_ADDR", "localhost:50051")
	userAddr := getenv("USER_SERVICE_ADDR", "localhost:50052")
	payAddr := getenv("PAYMENT_SERVICE_ADDR", "localhost:50053")

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

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
