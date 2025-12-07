package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Connect to Dragonfly via Consul DNS or localhost
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:16379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	// Test PING
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("PING failed: %v", err)
	}
	fmt.Printf("PING: %s\n", pong)

	// Test SET/GET
	err = client.Set(ctx, "styx:test", "hello from styx", time.Hour).Err()
	if err != nil {
		log.Fatalf("SET failed: %v", err)
	}
	fmt.Println("SET: OK")

	val, err := client.Get(ctx, "styx:test").Result()
	if err != nil {
		log.Fatalf("GET failed: %v", err)
	}
	fmt.Printf("GET: %s\n", val)

	// Test INCR
	client.Set(ctx, "styx:counter", "0", 0)
	for i := 0; i < 5; i++ {
		n, err := client.Incr(ctx, "styx:counter").Result()
		if err != nil {
			log.Fatalf("INCR failed: %v", err)
		}
		fmt.Printf("INCR: %d\n", n)
	}

	// Test hash operations
	err = client.HSet(ctx, "styx:config", map[string]interface{}{
		"version": "1.0.0",
		"env":     "dev",
	}).Err()
	if err != nil {
		log.Fatalf("HSET failed: %v", err)
	}
	fmt.Println("HSET: OK")

	config, err := client.HGetAll(ctx, "styx:config").Result()
	if err != nil {
		log.Fatalf("HGETALL failed: %v", err)
	}
	fmt.Printf("HGETALL: %v\n", config)

	// Cleanup
	client.Del(ctx, "styx:test", "styx:counter", "styx:config")
	fmt.Println("\nAll tests passed!")
}
