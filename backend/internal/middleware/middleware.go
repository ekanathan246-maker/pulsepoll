package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"pulsepoll/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// MakeToken builds a signed JWT string and sets it on the response cookie.
func MakeToken(userID primitive.ObjectID, email string, cfg *config.Config) (string, error) {
	c := claims{
		UserID: userID.Hex(),
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pulsepoll",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString([]byte(cfg.JWTSecret))
}

func parseToken(tokenStr string, secret string) (*claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tok.Claims.(*claims); ok && tok.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// SetAuthCookie writes the JWT into an httpOnly cookie on the response.
func SetAuthCookie(c *gin.Context, token string, cfg *config.Config) {
	maxAge := 7 * 24 * 3600
	secure := cfg.IsProduction()
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	c.SetSameSite(sameSite)
	c.SetCookie(cfg.CookieName, token, maxAge, "/", "", secure, true)
}

// ClearAuthCookie removes the token cookie on logout.
func ClearAuthCookie(c *gin.Context, cfg *config.Config) {
	secure := cfg.IsProduction()
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	c.SetSameSite(sameSite)
	c.SetCookie(cfg.CookieName, "", -1, "/", "", secure, true)
}

// AuthRequired extracts the user identity from the cookie and injects
// it into the request context. Unauthenticated requests get a 401.
func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(cfg.CookieName)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := parseToken(cookie, cfg.JWTSecret)
		if err != nil || claims.UserID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		oid, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "malformed user id"})
			return
		}
		c.Set("user_id", oid)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

// OptionalAuth is like AuthRequired but doesn't abort if the cookie is missing
// – it just skips setting the user fields.
func OptionalAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(cfg.CookieName)
		if err != nil || cookie == "" {
			c.Next()
			return
		}
		claims, err := parseToken(cookie, cfg.JWTSecret)
		if err != nil || claims.UserID == "" {
			c.Next()
			return
		}
		oid, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			c.Next()
			return
		}
		c.Set("user_id", oid)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

// CORS returns a middleware that respects the configured allowed origins and
// lets the browser send/receive cookies across origins during local development.
// Any origin that matches the request's own Host is also accepted, because the
// app is served same-origin through the reverse proxy – so localhost, LAN IPs
// and the deployed domain all just work without configuration.
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range cfg.Frontends {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// Requests without an Origin header (curl, same-origin GETs in some
		// browsers) are never subject to CORS.
		if origin == "" {
			c.Next()
			return
		}
		if !allowed[origin] && !sameOrigin(c.Request, origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		c.Header("Access-Control-Max-Age", "86400")
		if strings.EqualFold(c.Request.Method, "OPTIONS") {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// sameOrigin reports whether the Origin header matches the Host of the
// request, i.e. the call was made from the same origin served by this host.
func sameOrigin(req *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, req.Host)
}
