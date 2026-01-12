package loadbalancer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobin_SingleTarget(t *testing.T) {
	lb := NewRoundRobin([]string{"http://localhost:8080"})

	// Verify same target returned every time
	for range 10 {
		assert.Equal(t, "http://localhost:8080", lb.Next())
	}
}

func TestRoundRobin_MultipleTargets(t *testing.T) {
	targets := []string{
		"http://localhost:8080",
		"http://localhost:8081",
		"http://localhost:8082",
	}
	lb := NewRoundRobin(targets)

	// Verify round-robin distribution
	for i := range 6 {
		expected := targets[i%3]
		assert.Equal(t, expected, lb.Next())
	}
}

func TestRoundRobin_EmptyTargets(t *testing.T) {
	lb := NewRoundRobin([]string{})
	assert.Equal(t, "", lb.Next())
}

func TestRoundRobin_Concurrency(t *testing.T) {
	targets := []string{
		"http://localhost:8080",
		"http://localhost:8081",
		"http://localhost:8082",
	}
	lb := NewRoundRobin(targets)

	var wg sync.WaitGroup
	results := make(chan string, 300)

	// Spawn 100 goroutines, each making 3 calls
	for range 100 {
		wg.Go(func() {
			for range 3 {
				results <- lb.Next()
			}
		})
	}

	wg.Wait()
	close(results)

	// Count distribution
	counts := make(map[string]int)
	for result := range results {
		counts[result]++
	}

	// Each target should be selected exactly 100 times
	// (300 total calls / 3 targets = 100 each)
	for _, target := range targets {
		assert.Equal(t, 100, counts[target], "Expected equal distribution for target: %s", target)
	}
}

func TestRoundRobin_Targets(t *testing.T) {
	targets := []string{
		"http://localhost:8080",
		"http://localhost:8081",
	}
	lb := NewRoundRobin(targets)

	// Verify Targets() returns all targets
	assert.ElementsMatch(t, targets, lb.Targets())
}

func TestNewRoundRobin_NilSlice(t *testing.T) {
	lb := NewRoundRobin(nil)
	assert.NotNil(t, lb)
	assert.Equal(t, "", lb.Next())
}
