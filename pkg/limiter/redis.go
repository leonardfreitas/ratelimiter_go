package limiter

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDatasource struct {
	client *redis.Client
	mux    sync.Mutex
}

func NewRedisDatasource(client *redis.Client) *RedisDatasource {
	return &RedisDatasource{client: client, mux: sync.Mutex{}}
}

func (d *RedisDatasource) Set(key string, data *ClientRateLimiter) error {
	jsonData, err := json.Marshal(data)

	if err != nil {
		return err
	}

	if err := d.client.Set(context.Background(), key, string(jsonData), 60*time.Minute).Err(); err != nil {
		return err
	}
	return nil
}

func (d *RedisDatasource) Get(key string) (*ClientRateLimiter, error) {
	data, err := d.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	var client *ClientRateLimiter
	if err = json.Unmarshal([]byte(data), &client); err != nil {
		return nil, err
	}

	client.Mux = sync.Mutex{}

	return client, nil
}

func (d *RedisDatasource) Has(key string) bool {
	client, _ := d.Get(key)
	return client != nil
}

func (d *RedisDatasource) All() (map[string]*ClientRateLimiter, error) {
	keys, err := d.client.Keys(context.Background(), "*").Result()
	if err != nil {
		return nil, err
	}

	clients := make(map[string]*ClientRateLimiter)

	for _, key := range keys {
		client, err := d.Get(key)
		if err != nil {
			return nil, err
		}
		clients[key] = client
	}

	return clients, nil
}
