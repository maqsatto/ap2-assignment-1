package http

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RedisRateLimiter(client *redis.Client, limit int64, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()

		key := "rate_limit:" + clientIP(c)
		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			_ = client.Expire(ctx, key, window).Err()
		}

		c.Header("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(maxInt64(0, limit-count), 10))

		if count > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}

		c.Next()
	}
}

func clientIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return c.ClientIP()
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
