package loadbalancer

import (
	"sync/atomic"
)

// RoundRobin implements round-robin load balancing across multiple targets.
// It distributes requests evenly across all available targets using an atomic counter
// to ensure thread-safe operation without the need for locks.
type RoundRobin struct {
	targets []string
	counter uint64
}

// NewRoundRobin creates a new round-robin load balancer with the given targets.
// It returns a load balancer that will cycle through targets in order.
func NewRoundRobin(targets []string) *RoundRobin {
	if len(targets) == 0 {
		return &RoundRobin{targets: []string{}}
	}
	return &RoundRobin{
		targets: targets,
		counter: 0,
	}
}

// Next returns the next target using round-robin algorithm.
// This method is thread-safe and uses atomic operations for concurrent access.
// Returns empty string if no targets are available.
func (r *RoundRobin) Next() string {
	if len(r.targets) == 0 {
		return ""
	}

	// Fast path for single target
	if len(r.targets) == 1 {
		return r.targets[0]
	}

	// Atomic increment and modulo to get index
	idx := (atomic.AddUint64(&r.counter, 1) - 1) % uint64(len(r.targets))
	return r.targets[idx]
}
