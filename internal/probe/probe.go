package probe

import (
	"context"
	"time"
)

type Runner interface {
	Run(ctx context.Context) error
}

func IntervalFromRPS(rps float64) time.Duration {
	if rps <= 0 {
		return time.Second
	}

	interval := time.Duration(float64(time.Second) / rps)
	if interval < time.Millisecond {
		return time.Millisecond
	}
	return interval
}
