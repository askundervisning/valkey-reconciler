package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	if redisHost == "" {
		redisHost = "localhost"
	}

	if redisPort == "" {
		redisPort = "6379"
	}

	if redisPassword == "" {
		log.Fatal("REDIS_PASSWORD is not set")
	}

	if redisPassword == "" {
		log.Fatal("REDIS_PASSWORD is not set")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		ctx := context.Background()

		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})

		value := r.URL.Query().Get("value")

		val, err := rdb.Get(ctx, "key").Result()
		if err != nil {
			if value == "0" {
				val = "0"
			} else {
				log.Printf("Error getting key: %s", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("NOT OK"))
				return
			}
		}

		err = rdb.Set(ctx, "key", value, 0).Err()
		if err != nil {
			log.Printf("Error setting key: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("NOT OK"))
			return
		}

		log.Printf("Returning success: %s", value)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK - " + val))
	})
	fmt.Println("Starting testserver on port", port)
	fmt.Println("Redis host:port", redisHost+":"+redisPort)

	http.ListenAndServe(":"+port, nil)
}
