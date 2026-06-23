package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter uses a token bucket per client ip.
type RateLimiter struct {
	rate    int
	burst   int
	buckets sync.Map
	cleanup time.Duration
}

// NewRateLimiter creates a per-ip token bucket limiter.
func NewRateLimiter(rps, burst int) *RateLimiter {
	return &RateLimiter{
		rate:    rps,
		burst:   burst,
		cleanup: 10 * time.Minute,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	if v, ok := rl.buckets.Load(ip); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(rate.Limit(rl.rate), rl.burst)
	actual, _ := rl.buckets.LoadOrStore(ip, lim)
	return actual.(*rate.Limiter)
}

// Middleware returns gin middleware enforcing per-ip rate limits.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.getLimiter(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// NewRateLimitMiddleware is a convenience wrapper matching the phase 3 spec signature.
func NewRateLimitMiddleware(rps, burst int) gin.HandlerFunc {
	return NewRateLimiter(rps, burst).Middleware()
}
