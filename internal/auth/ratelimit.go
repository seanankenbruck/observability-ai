// internal/auth/ratelimit.go
package auth

import (
	"sync"
	"time"
)

// ClientLimiter tracks requests for a single client
type ClientLimiter struct {
	requests  []time.Time
	mutex     sync.Mutex
	lastClean time.Time
}

// RateLimiter provides in-memory rate limiting with sliding window
type RateLimiter struct {
	clients map[string]*ClientLimiter
	mutex   sync.RWMutex
}

var (
	globalRateLimiter *RateLimiter
	once              sync.Once
)

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*ClientLimiter),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request should be allowed based on rate limit
func (rl *RateLimiter) Allow(clientID string, limitPerMinute int) bool {
	rl.mutex.Lock()
	client, exists := rl.clients[clientID]
	if !exists {
		client = &ClientLimiter{
			requests:  make([]time.Time, 0),
			lastClean: time.Now(),
		}
		rl.clients[clientID] = client
	}
	rl.mutex.Unlock()

	return client.allow(limitPerMinute)
}

// allow checks and records a request for a client
func (cl *ClientLimiter) allow(limitPerMinute int) bool {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Minute)

	// Clean old requests
	cl.cleanOldRequests(windowStart)

	// Check if limit is exceeded
	if len(cl.requests) >= limitPerMinute {
		return false
	}

	// Record this request
	cl.requests = append(cl.requests, now)
	cl.lastClean = now

	return true
}

// cleanOldRequests removes requests older than the window
func (cl *ClientLimiter) cleanOldRequests(windowStart time.Time) {
	validRequests := make([]time.Time, 0, len(cl.requests))
	for _, req := range cl.requests {
		if req.After(windowStart) {
			validRequests = append(validRequests, req)
		}
	}
	cl.requests = validRequests
}

// cleanup removes inactive clients (no requests in last 5 minutes)
func (rl *RateLimiter) cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)

	for clientID, client := range rl.clients {
		client.mutex.Lock()
		if client.lastClean.Before(cutoff) {
			delete(rl.clients, clientID)
		}
		client.mutex.Unlock()
	}
}

// cleanupLoop runs periodic cleanup
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// GetStats returns rate limiting statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_clients"] = len(rl.clients)

	clientStats := make([]map[string]interface{}, 0, len(rl.clients))
	for clientID, client := range rl.clients {
		client.mutex.Lock()
		clientStats = append(clientStats, map[string]interface{}{
			"client_id":      clientID,
			"request_count":  len(client.requests),
			"last_request":   client.lastClean,
		})
		client.mutex.Unlock()
	}
	stats["clients"] = clientStats

	return stats
}

// Global rate limiter instance

// GetGlobalRateLimiter returns the singleton rate limiter instance
func GetGlobalRateLimiter() *RateLimiter {
	once.Do(func() {
		globalRateLimiter = NewRateLimiter()
	})
	return globalRateLimiter
}

// CheckRateLimit checks if a request should be allowed (convenience function)
func CheckRateLimit(clientID string, limitPerMinute int) bool {
	return GetGlobalRateLimiter().Allow(clientID, limitPerMinute)
}

// GetRateLimitStats returns rate limiting statistics (convenience function)
func GetRateLimitStats() map[string]interface{} {
	return GetGlobalRateLimiter().GetStats()
}
