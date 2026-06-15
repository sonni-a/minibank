package redis

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func Connect(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	log.Println("Connected to Redis:", addr)
	return rdb
}
