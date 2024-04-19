package main

import (
	"net/http"
	"time"

	"github.com/leonardfreitas/ratelimiter_go/configs"
	"github.com/leonardfreitas/ratelimiter_go/internal/http/middlewares"
	"github.com/leonardfreitas/ratelimiter_go/pkg/limiter"
	"github.com/redis/go-redis/v9"
)

func listOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("list of orders"))
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
