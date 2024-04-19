package limiter

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrMaxRequests            = errors.New("you have reached the maximum number of requests or actions allowed within a certain time frame")
	ErrGettingRateLimiterData = errors.New("error getting rate limiter data from the datasource")
	ErrNilConfig              = errors.New("config cannot be nil")
)

type Sleeper interface {
	Sleep(d time.Duration)
}

type Datasource interface {
	Set(key string, data *ClientRateLimiter) error
	Get(key string) (*ClientRateLimiter, error)
	Has(key string) bool
	All() (map[string]*ClientRateLimiter, error)
}

type RateLimiter struct {
	datasource Datasource
	sleeper    Sleeper
}

type BaseLimiterConfig struct {
	RequestesPerSecond int
	BlockUserFor       time.Duration
}

type RateLimiterConfigByIP struct {
	BaseLimiterConfig
}

type RateLimiterConfigByToken struct {
	BaseLimiterConfig
	Key string
}

type RateLimiterConfig struct {
	ConfigByIP    *RateLimiterConfigByIP
	ConfigByToken *RateLimiterConfigByToken
}

func NewRateLimiterConfigByIP(requestesPerSecond int, blockUserFor time.Duration) *RateLimiterConfigByIP {
	return &RateLimiterConfigByIP{
		BaseLimiterConfig: BaseLimiterConfig{
			RequestesPerSecond: requestesPerSecond,
			BlockUserFor:       blockUserFor,
		},
	}
}

func NewRateLimiterConfigByToken(requestesPerSecond int, blockUserFor time.Duration, key string) *RateLimiterConfigByToken {
	return &RateLimiterConfigByToken{
		BaseLimiterConfig: BaseLimiterConfig{
			RequestesPerSecond: requestesPerSecond,
			BlockUserFor:       blockUserFor,
		},
		Key: key,
	}
}

func NewRateLimiterConfig(configByIP *RateLimiterConfigByIP, configByToken *RateLimiterConfigByToken) *RateLimiterConfig {
	return &RateLimiterConfig{
		ConfigByIP:    configByIP,
		ConfigByToken: configByToken,
	}
}

func NewRateLimiter(datasource Datasource, sleeper Sleeper) *RateLimiter {
	limiter := &RateLimiter{datasource: datasource, sleeper: sleeper}

	go limiter.clearRequests()

	return limiter
}

func (r *RateLimiter) clear() {
	r.sleeper.Sleep(1 * time.Second)
	clients, err := r.datasource.All()
	if err != nil {
		panic(err)
	}
	for key, client := range clients {
		client.Mux.Lock()
		defer client.Mux.Unlock()
		client.clearRequests()
		if client.hasBlockingExpired() {
			client.resetBlock()
		}
		r.datasource.Set(key, client)
	}
}

func (r *RateLimiter) clearRequests() {
	for {
		r.clear()
	}
}

func (r *RateLimiter) setConfigBy(key string, config *BaseLimiterConfig) (*ClientRateLimiter, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	var client *ClientRateLimiter

	if found := r.datasource.Has(key); !found {
		client = newClientLimiter(config.RequestesPerSecond, config.BlockUserFor)
		if err := r.datasource.Set(key, client); err != nil {
			return nil, err
		}
		return client, nil
	}

	client, err := r.datasource.Get(key)
	if err != nil {
		return nil, err
	}

	if client.RequestsPerSecond != config.RequestesPerSecond {
		client.RequestsPerSecond = config.RequestesPerSecond
		client.BlockUserFor = config.BlockUserFor
		if err := r.datasource.Set(key, client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (r *RateLimiter) getClient(ip, token string, config *RateLimiterConfig) (*ClientRateLimiter, string, error) {
	if config == nil {
		return nil, "", ErrNilConfig
	}

	ipConfig, tokenConfig := config.ConfigByIP, config.ConfigByToken

	mux := sync.Mutex{}
	mux.Lock()
	defer mux.Unlock()

	var client *ClientRateLimiter
	var key string

	if ipConfig != nil {
		ipClient, err := r.setConfigBy(ip, &ipConfig.BaseLimiterConfig)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %w", err, ErrGettingRateLimiterData)
		}
		client = ipClient
		key = ip
	}

	if token != "" && tokenConfig != nil {
		tokenClient, err := r.setConfigBy(token, &tokenConfig.BaseLimiterConfig)

		if err != nil {
			return nil, "", fmt.Errorf("%w: %w", err, ErrGettingRateLimiterData)
		}

		client = tokenClient
		key = token
	}

	return client, key, nil
}

func (r *RateLimiter) HandleRequest(ip, token string, config *RateLimiterConfig) error {
	client, key, err := r.getClient(ip, token, config)

	if err != nil {
		return ErrGettingRateLimiterData
	}

	err = client.verifyAndBlockUser(r.datasource, key)

	if err != nil {
		return err
	}

	return nil
}

type ClientRateLimiter struct {
	RequestsPerSecond int           `json:"requestsPerSecond"`
	BlockUserFor      time.Duration `json:"blockUserFor"`
	Blocked           bool          `json:"blocked"`
	BlockedAt         time.Time     `json:"blockedAt"`
	TotalRequests     int           `json:"totalRequests"`
	Mux               sync.Mutex    `json:"-"`
}

func newClientLimiter(rps int, blockDuration time.Duration) *ClientRateLimiter {
	return &ClientRateLimiter{
		RequestsPerSecond: rps,
		BlockUserFor:      blockDuration,
		Mux:               sync.Mutex{},
	}
}

func (c *ClientRateLimiter) verifyAndBlockUser(datasource Datasource, key string) error {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	if c.isBlocked() {
		if c.hasBlockingExpired() {
			c.resetBlock()
			if err := datasource.Set(key, c); err != nil {
				return err
			}
		} else {
			return ErrMaxRequests
		}
	}

	c.TotalRequests += 1

	if c.shouldBlock() {
		c.block()
		if err := datasource.Set(key, c); err != nil {
			return err
		}
		return ErrMaxRequests
	}

	if err := datasource.Set(key, c); err != nil {
		return err
	}

	return nil
}

func (c *ClientRateLimiter) clearRequests() {
	if c.TotalRequests <= c.RequestsPerSecond {
		c.TotalRequests = 0
	}
}

func (c *ClientRateLimiter) isBlocked() bool {
	return c.Blocked
}

func (c *ClientRateLimiter) hasBlockingExpired() bool {
	return time.Since(c.BlockedAt) > c.BlockUserFor
}

func (c *ClientRateLimiter) resetBlock() {
	c.Blocked = false
	c.TotalRequests = 0
	c.BlockedAt = time.Time{}
}

func (c *ClientRateLimiter) shouldBlock() bool {
	return c.TotalRequests > c.RequestsPerSecond
}

func (c *ClientRateLimiter) block() {
	c.Blocked = true
	c.BlockedAt = time.Now()
}
