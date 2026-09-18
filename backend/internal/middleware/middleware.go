package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"pulsepoll/backend/internal/auth"
	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/models"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "pp_csrf"

func SetSessionCookies(c *gin.Context, credentials auth.Credentials, cfg *config.Config) {
	secure := cfg.IsProduction()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cfg.CookieName, credentials.Token, 7*24*3600, "/", "", secure, true)
	c.SetCookie(csrfCookieName, credentials.CSRFToken, 7*24*3600, "/", "", secure, false)
}

func ClearSessionCookies(c *gin.Context, cfg *config.Config) {
	secure := cfg.IsProduction()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cfg.CookieName, "", -1, "/", "", secure, true)
	c.SetCookie(csrfCookieName, "", -1, "/", "", secure, false)
}

func AuthRequired(sessions *auth.Manager, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(cfg.CookieName)
		session, err := sessions.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorBody("authentication_required", "Sign in to continue."))
			return
		}
		c.Set("user_id", session.UserID)
		c.Set("session", session)
		c.Next()
	}
}

func OptionalAuth(sessions *auth.Manager, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(cfg.CookieName)
		if session, err := sessions.Authenticate(c.Request.Context(), token); err == nil {
			c.Set("user_id", session.UserID)
			c.Set("session", session)
		}
		c.Next()
	}
}

// CSRFRequired protects cookie-authenticated state changes. It must run after
// AuthRequired so the verified session is available in context.
func CSRFRequired(sessions *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get("session")
		session, valid := value.(models.Session)
		if !ok || !valid || !sessions.ValidCSRF(session, c.GetHeader("X-CSRF-Token")) {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorBody("csrf_invalid", "Refresh the page and try again."))
			return
		}
		c.Next()
	}
}

// CORS uses an exact allowlist. Same-origin calls are accepted so the combined
// production image and local reverse proxy need no wildcard.
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range cfg.Frontends {
		allowed[strings.TrimRight(origin, "/")] = true
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin == "" {
			c.Next()
			return
		}
		if !allowed[origin] && !sameOrigin(c.Request, origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorBody("origin_not_allowed", "This origin is not allowed."))
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,X-CSRF-Token,Idempotency-Key,X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID,Retry-After")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func sameOrigin(req *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, req.Host)
}

func ErrorBody(code, message string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": message}}
}
