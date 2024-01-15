package prober

import "time"

func calculateInterval(rps float64) time.Duration {
	return time.Duration(1000.0/rps) * time.Millisecond
}
