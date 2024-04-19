package limiter

import "time"

type TimeSleeper struct{}

func NewTimeSleeper() *TimeSleeper {
	return &TimeSleeper{}
}

func (t *TimeSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}
