package balancer

import (
	"testing"
)

func TestBalancer_RoundRobinAndFailover(t *testing.T) {
	// 1. Setup a mock health tracker where node 1 is healthy and node 2 is dead
	targets := []string{"http://node-1:8080", "http://node-2:8080"}
	
	// 2. Instantiate your balancer and inject the health state
	// Verify b.Pick() only returns "http://node-1:8080"
}