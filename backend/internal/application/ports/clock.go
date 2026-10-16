package ports

import "time"

// Clock provides current time in UTC.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using time.Now().UTC().
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
