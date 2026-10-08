package http

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/earnmart/earnmart-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		slog.Info("http request",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(started).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func cors(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		_, isAllowed := allowed[origin]
		if origin != "" && (allowAll || isAllowed) {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			if origin != "" && !allowAll && !isAllowed {
				failure(c, http.StatusForbidden, "CORS_FORBIDDEN", "Origin is not allowed", nil)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func secureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func authenticate(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			failure(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication is required", nil)
			return
		}
		user, claims, err := auth.ValidateAccess(c.Request.Context(), token)
		if err != nil {
			domainFailure(c, err)
			return
		}
		c.Set("auth_user_id", user.ID)
		c.Set("auth_session_id", claims.SessionID)
		c.Next()
	}
}

func maintenanceGate(enabled bool, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enabled {
			failure(c, http.StatusServiceUnavailable, "MAINTENANCE_MODE", message, nil)
			return
		}
		c.Next()
	}
}

func rateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	type visitor struct {
		count   int
		resetAt time.Time
	}

	var mu sync.Mutex
	visitors := make(map[string]visitor)

	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()

		mu.Lock()
		entry, found := visitors[key]
		if !found || !now.Before(entry.resetAt) {
			entry = visitor{resetAt: now.Add(window)}
		}
		entry.count++
		visitors[key] = entry
		mu.Unlock()

		if entry.count > maxRequests {
			retryAfter := int(time.Until(entry.resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			failure(c, http.StatusTooManyRequests, "RATE_LIMITED", "Bạn thao tác quá nhanh. Vui lòng thử lại sau.", nil)
			return
		}
		c.Next()
	}
}
