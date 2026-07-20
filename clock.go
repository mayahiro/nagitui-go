package tui

import (
	"math"
	"sync/atomic"
	"time"
)

// Timestamp is a monotonic runtime time measured in nanoseconds from a clock
// origin
type Timestamp uint64

// Nanoseconds returns nanoseconds since the clock origin
func (t Timestamp) Nanoseconds() uint64 {
	return uint64(t)
}

// Add returns a timestamp advanced by duration, saturating at the maximum
//
// A nonpositive duration leaves the timestamp unchanged.
func (t Timestamp) Add(duration time.Duration) Timestamp {
	if duration <= 0 {
		return t
	}
	return Timestamp(saturatingAdd64(uint64(t), uint64(duration)))
}

// Clock is a monotonic time source used by runtime scheduling
type Clock interface {
	// Now returns the current monotonic timestamp
	Now() Timestamp
}

// SystemClock is a production monotonic clock based on time.Time
type SystemClock struct {
	origin time.Time
}

// NewSystemClock returns a production clock with a new origin
func NewSystemClock() SystemClock {
	return SystemClock{origin: time.Now()}
}

// Now returns elapsed monotonic time from the clock origin
func (c SystemClock) Now() Timestamp {
	elapsed := time.Since(c.origin)
	if elapsed <= 0 {
		return 0
	}
	return Timestamp(elapsed)
}

// VirtualClock is a manually advanced monotonic clock for deterministic tests
type VirtualClock struct {
	nanoseconds atomic.Uint64
}

// NewVirtualClock returns a virtual clock at timestamp zero
func NewVirtualClock() *VirtualClock {
	return &VirtualClock{}
}

// Now returns the current virtual timestamp
func (c *VirtualClock) Now() Timestamp {
	return Timestamp(c.nanoseconds.Load())
}

// Advance moves the clock forward and returns its new timestamp
//
// A nonpositive duration leaves the clock unchanged.
func (c *VirtualClock) Advance(duration time.Duration) Timestamp {
	if duration <= 0 {
		return c.Now()
	}
	delta := uint64(duration)
	for {
		current := c.nanoseconds.Load()
		next := current + delta
		if next < current {
			next = math.MaxUint64
		}
		if c.nanoseconds.CompareAndSwap(current, next) {
			return Timestamp(next)
		}
	}
}
