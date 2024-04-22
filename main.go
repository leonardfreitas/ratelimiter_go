package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/leonardfreitas/ratelimiter_go/configs"
	"github.com/leonardfreitas/ratelimiter_go/internal/http/middlewares"
	"github.com/leonardfreitas/ratelimiter_go/pkg/limiter"
	"github.com/redis/go-redis/v9"
)

type Response struct {
	Success bool `json:"success"`
}

func listOrders(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Success: true,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func main() {
	envConf, err := configs.LoadConfig(".")

	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     envConf.RedisHost,
		Password: envConf.RedisPassword,
		DB:       envConf.RedisDB,
	})

	rateLimiterConf := limiter.NewRateLimiterConfig(
		limiter.NewRateLimiterConfigByIP(envConf.MaxRequestsByIP, time.Duration(envConf.BlockUserForByIP)*time.Second),
		limiter.NewRateLimiterConfigByToken(envConf.MaxRequestsByToken, time.Duration(envConf.BlockUserForByToken)*time.Second, "API_KEY"),
	)
	http.Handle("/", middlewares.RateLimiter(listOrders, rateLimiterConf, redisClient))
	http.ListenAndServe(":8080", nil)
}
