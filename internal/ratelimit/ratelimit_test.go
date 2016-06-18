package ratelimit

import (
	"testing"
	"time"
)

func TestWait(t *testing.T) {
	// 2 per second so we should be waiting 500 milliseconds.
	l := New(2, time.Second, 0)
	now := time.Now()
	limit, wait := l.Try()
	if !limit {
		t.Fatalf("expected we need to limit")
	}
	time.Sleep(wait)
	waited := time.Now().Sub(now)

	if waited > time.Millisecond*510 || waited < time.Millisecond*490 {
		t.Fatalf("waited for %v instead of %v", waited, time.Millisecond*500)
	}
}
