package limiter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type DatasourceMock struct {
	mock.Mock
}

func (m *DatasourceMock) Set(key string, data *ClientRateLimiter) error {
	args := m.Called(key, data)
	return args.Error(0)
}

func (m *DatasourceMock) Get(key string) (*ClientRateLimiter, error) {
	args := m.Called(key)
	return args.Get(0).(*ClientRateLimiter), args.Error(1)
}

func (m *DatasourceMock) Has(key string) bool {
	args := m.Called(key)
	return args.Bool(0)
}

func (m *DatasourceMock) All() (map[string]*ClientRateLimiter, error) {
	args := m.Called()
	return args.Get(0).(map[string]*ClientRateLimiter), args.Error(1)
}

type TimeSleeperMock struct {
	mock.Mock
}

func (m *TimeSleeperMock) Sleep(d time.Duration) {
	m.Called(d)
}

func TestHandleRequest(t *testing.T) {
	t.Run("setConfigBy", func(t *testing.T) {
		t.Run("should return an error when the config is nil", func(t *testing.T) {
			limiter := NewRateLimiter(NewInMemoryDatasource(), NewTimeSleeper())
			_, err := limiter.setConfigBy("key", nil)
			assert.NotNil(t, err)
			assert.ErrorIs(t, err, ErrNilConfig)
		})
		t.Run("should set the client config in the datasource when it's not found", func(t *testing.T) {
			key := "127.0.0.1"

			datasource := NewInMemoryDatasource()
			client, _ := datasource.Get(key)
			assert.Nil(t, client)

			limiter := NewRateLimiter(datasource, NewTimeSleeper())

			client, err := limiter.setConfigBy(key, &BaseLimiterConfig{
				RequestesPerSecond: 10,
				BlockUserFor:       10 * time.Second,
			})

			assert.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, 10, client.RequestsPerSecond)
			assert.Equal(t, 10*time.Second, client.BlockUserFor)

			datasourceClient, _ := datasource.Get(key)
			assert.Equal(t, client, datasourceClient)
		})
		t.Run("should use the same config from the datasource when it's found, and update its value if changed", func(t *testing.T) {
			key := "127.0.0.1"

			clientLimiter := newClientLimiter(5, 5*time.Second)

			datasource := NewInMemoryDatasource()
			err := datasource.Set(key, clientLimiter)
			assert.NoError(t, err)

			limiter := NewRateLimiter(datasource, NewTimeSleeper())

			client, err := limiter.setConfigBy(key, &BaseLimiterConfig{
				RequestesPerSecond: 10,
				BlockUserFor:       10 * time.Second,
			})
			assert.NoError(t, err)

			assert.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, 10, client.RequestsPerSecond)
			assert.Equal(t, 10*time.Second, client.BlockUserFor)

			datasourceClient, _ := datasource.Get(key)
			assert.Equal(t, client, datasourceClient)
		})
		t.Run("should return an error when failing setting the config", func(t *testing.T) {
			key := "127.0.0.1"

			errNotFound := errors.New("not found")

			datasource := &DatasourceMock{}
			datasource.On("Has", key).Return(true)
			datasource.On("Get", key).Return(&ClientRateLimiter{}, errNotFound)

			limiter := NewRateLimiter(datasource, NewTimeSleeper())

			client, err := limiter.setConfigBy(key, &BaseLimiterConfig{
				RequestesPerSecond: 10,
				BlockUserFor:       10 * time.Second,
			})

			assert.Nil(t, client)
			assert.Error(t, err)
			assert.IsType(t, errNotFound, err)
		})
	})

	t.Run("getClient", func(t *testing.T) {
		t.Run("should return an error if none of the availables configs are provided", func(t *testing.T) {
			ip := "127.0.0.1"

			datasource := &DatasourceMock{}
			limiter := NewRateLimiter(datasource, NewTimeSleeper())

			client, key, err := limiter.getClient(ip, "", nil)

			assert.Nil(t, client)
			assert.Empty(t, key)
			assert.ErrorIs(t, err, ErrNilConfig)
		})
		t.Run("should set the config by ip when the token config is not set", func(t *testing.T) {
			ip := "127.0.0.1"
			token := ""

			clientLimiter := newClientLimiter(10, 10*time.Second)

			datasource := &DatasourceMock{}
			datasource.On("Get", ip).Return(clientLimiter, nil)
			datasource.On("Has", ip).Return(true)

			limiter := NewRateLimiter(datasource, NewTimeSleeper())

			client, key, err := limiter.getClient(ip, token, NewRateLimiterConfig(
				NewRateLimiterConfigByIP(10, 10*time.Second), nil,
			))

			assert.NoError(t, err)
			assert.Equal(t, ip, key)
			assert.NotNil(t, client)
			assert.Equal(t, clientLimiter, client)
		})

		t.Run("should set the config by token when both configs are provided", func(t *testing.T) {
			ip := "127.0.0.1"
			token := "abc1234"

			clientIpLimiter := newClientLimiter(10, 10*time.Second)
			clientTokenLimiter := newClientLimiter(5, 30*time.Second)

			datasourceMock := &DatasourceMock{}

			datasourceMock.On("Get", ip).Return(clientIpLimiter, nil).Once().On("Has", ip).Return(true).Once()
			datasourceMock.On("Get", token).Return(clientTokenLimiter, nil).Once().On("Has", token).Return(true).Once()

			limiter := NewRateLimiter(datasourceMock, NewTimeSleeper())

			client, key, err := limiter.getClient(ip, token, NewRateLimiterConfig(
				NewRateLimiterConfigByIP(10, 10*time.Second),
				NewRateLimiterConfigByToken(5, 30*time.Second, "API_KEY"),
			))

			assert.NoError(t, err)
			assert.Equal(t, token, key)
			assert.NotNil(t, client)
			assert.Equal(t, clientTokenLimiter, client)
		})

		t.Run("should set the config by token when it's the only config provided", func(t *testing.T) {
			ip := "127.0.0.1"
			token := "abc1234"

			clientIpLimiter := newClientLimiter(10, 10*time.Second)
			clientTokenLimiter := newClientLimiter(5, 30*time.Second)

			datasourceMock := &DatasourceMock{}

			datasourceMock.On("Get", ip).Return(clientIpLimiter, nil).Once().On("Has", ip).Return(true).Once()
			datasourceMock.On("Get", token).Return(clientTokenLimiter, nil).Once().On("Has", token).Return(true).Once()

			limiter := NewRateLimiter(datasourceMock, NewTimeSleeper())

			client, key, err := limiter.getClient(ip, token, NewRateLimiterConfig(
				nil,
				NewRateLimiterConfigByToken(5, 30*time.Second, "API_KEY"),
			))

			assert.NoError(t, err)
			assert.Equal(t, token, key)
			assert.NotNil(t, client)
			assert.Equal(t, clientTokenLimiter, client)
		})
	})

	t.Run("should return nil when the request is allowed", func(t *testing.T) {
		ip := "127.0.0.1"

		config := NewRateLimiterConfig(
			NewRateLimiterConfigByIP(10, 10*time.Second), nil,
		)

		datasource := &DatasourceMock{}
		datasource.
			On("Get", ip).
			Return(nil, errors.New("not found")).
			On("Has", ip).
			Return(false).
			On("Set", ip, mock.Anything).
			Return(nil)

		limiter := NewRateLimiter(datasource, NewTimeSleeper())

		err := limiter.HandleRequest(ip, "", config)

		assert.NoError(t, err)
	})

	t.Run("should block the user after 10 requests/second", func(t *testing.T) {
		ip := "127.0.0.1"

		datasource := NewInMemoryDatasource()
		limiter := NewRateLimiter(datasource, NewTimeSleeper())
		config := NewRateLimiterConfig(
			NewRateLimiterConfigByIP(10, 10*time.Second), nil,
		)

		for i := 0; i < 10; i++ {
			err := limiter.HandleRequest(ip, "", config)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		}

		err := limiter.HandleRequest(ip, "", config)

		assert.ErrorIs(t, err, ErrMaxRequests)
	})

	t.Run("should block the user after 5 requests/second", func(t *testing.T) {
		ip := "127.0.0.1"
		token := "abc1234"

		datasource := NewInMemoryDatasource()
		limiter := NewRateLimiter(datasource, NewTimeSleeper())
		config := NewRateLimiterConfig(
			NewRateLimiterConfigByIP(10, 10*time.Second),
			NewRateLimiterConfigByToken(5, 30*time.Second, "API_KEY"),
		)

		for i := 0; i < 5; i++ {
			err := limiter.HandleRequest(ip, token, config)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		}

		err := limiter.HandleRequest(ip, token, config)

		assert.ErrorIs(t, err, ErrMaxRequests)
	})

	t.Run("should unblock the user when the blocking period has expired", func(t *testing.T) {
		ip := "127.0.0.1"

		timeSleeper := &TimeSleeperMock{}
		timeSleeper.On("Sleep", 1*time.Second).Return()

		client := newClientLimiter(5, 30*time.Second)
		client.Blocked = true
		client.BlockedAt = time.Now().Add(-31 * time.Second)
		client.TotalRequests = 10

		assert.True(t, client.isBlocked())

		datasource := NewInMemoryDatasource()
		err := datasource.Set(ip, client)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		limiter := NewRateLimiter(datasource, timeSleeper)

		limiter.clear()

		client, err = datasource.Get(ip)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		assert.False(t, client.isBlocked())
		assert.Equal(t, 0, client.TotalRequests)
	})
}
