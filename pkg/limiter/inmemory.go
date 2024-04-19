package limiter

import (
	"sync"
)

type InMemoryDatasource struct {
	clients map[string]*ClientRateLimiter
	mux     sync.Mutex
}

func NewInMemoryDatasource() *InMemoryDatasource {
	return &InMemoryDatasource{clients: make(map[string]*ClientRateLimiter), mux: sync.Mutex{}}
}

func (d *InMemoryDatasource) Set(key string, data *ClientRateLimiter) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	d.clients[key] = data
	return nil
}

func (d *InMemoryDatasource) Get(key string) (*ClientRateLimiter, error) {
	if data, found := d.clients[key]; found {
		return data, nil
	}
	return nil, nil
}

func (d *InMemoryDatasource) Has(key string) bool {
	_, found := d.clients[key]
	return found
}

func (d *InMemoryDatasource) All() (map[string]*ClientRateLimiter, error) {
	return d.clients, nil
}
