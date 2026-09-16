package config

import (
	"os"
	"strings"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	Port       string
	Env        string
	MongoURI   string
	MongoDB    string
	RedisAddr  string
	RedisPass  string
	JWTSecret  string
	Frontends  []string
	CookieName string
	PublicURL  string
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvs(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		var out []string
		for _, part := range splitComma(v) {
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	return fallback
}

func splitComma(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// Load builds a Config from the environment.
func Load() *Config {
	return &Config{
		Port:       getenv("PORT", "8080"),
		Env:        getenv("ENV", "development"),
		MongoURI:   getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:    getenv("MONGO_DB", "pulsepoll"),
		RedisAddr:  getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:  getenv("REDIS_PASS", ""),
		JWTSecret:  getenv("JWT_SECRET", "dev-secret-change-me"),
		Frontends:  getenvs("FRONTEND_ORIGINS", []string{"http://localhost:5173", "http://localhost"}),
		CookieName: "pp_token",
		PublicURL:  strings.TrimRight(getenv("PUBLIC_URL", ""), "/"),
	}
}

// IsProduction reports whether the app runs in a production environment.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
