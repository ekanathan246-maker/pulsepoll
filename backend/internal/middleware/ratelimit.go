package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var fixedWindowScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return count
`)

type IdentityFunc func(*gin.Context) string

// RateLimit intentionally fails open when Redis is unavailable: Redis protects
// the service, but it is never the acceptance boundary for a durable vote.
func RateLimit(client *redis.Client, route string, limit int64, window time.Duration, identity IdentityFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucket := time.Now().UTC().Unix() / int64(window/time.Second)
		key := "pp:rl:" + route + ":" + digest(identity(c)) + ":" + strconv.FormatInt(bucket, 10)
		count, err := fixedWindowScript.Run(c.Request.Context(), client, []string{key}, int64(window/time.Second)).Int64()
		if err == nil && count > limit {
			c.Header("Retry-After", strconv.FormatInt(int64(window/time.Second), 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorBody("rate_limited", "Too many requests. Wait a moment and try again."))
			return
		}
		c.Next()
	}
}

func IPIdentity(c *gin.Context) string { return c.ClientIP() }

func VoterIdentity(c *gin.Context) string {
	voter, _ := c.Cookie("ppv")
	return c.ClientIP() + ":" + voter
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}
