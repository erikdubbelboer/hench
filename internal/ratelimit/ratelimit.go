package ratelimit

import (
	"math"
	"sync"
	"time"
)

type Limiter struct {
	rate float64
	per  float64

	m    sync.Mutex
	left float64
	last time.Time
}

// New creates a new ratelimiter that allows `rate` actions per `per` duration.
// The ratelimiter starts with `start` allowed actions.
func New(rate float64, per time.Duration, start float64) *Limiter {
	return &Limiter{
		rate: rate,
		per:  float64(per),
		left: start,
		last: time.Now(),
	}
}

// Limit returns true if NO action should be performed.
func (l *Limiter) Limit() bool {
	limit := false
	now := time.Now()

	l.m.Lock()

	d := float64(now.Sub(l.last))
	l.last = now
	l.left += l.rate * (d / l.per)

	if l.left > l.rate {
		l.left = l.rate - 1
	} else if l.left >= 1 {
		l.left -= 1
	} else {
		limit = true
	}

	l.m.Unlock()

	return limit
}

// Set sets the how many actions can be performed in which duration.
func (l *Limiter) Set(rate float64, per time.Duration) {
	l.m.Lock()

	l.rate = rate
	l.per = float64(per)

	l.m.Unlock()
}

// Left returns how many actions this limiter has left.
func (l *Limiter) Left() float64 {
	left := float64(0)
	now := time.Now()

	l.m.Lock()

	d := float64(now.Sub(l.last))
	l.last = now
	l.left += l.rate * (d / l.per)

	left = l.left

	l.m.Unlock()

	return left
}

func (l *Limiter) Try() (bool, time.Duration) {
	var duration time.Duration

	limit := false
	now := time.Now()

	l.m.Lock()

	d := float64(now.Sub(l.last))
	l.last = now
	l.left += l.rate * (d / l.per)

	if l.left > l.rate {
		l.left = l.rate - 1
	} else if l.left >= 1 {
		l.left -= 1
	} else {
		limit = true

		needs := 1 - l.left
		seconds := math.Ceil(needs / (l.rate / l.per))
		duration = time.Duration(seconds)
	}

	l.m.Unlock()

	return limit, duration
}
