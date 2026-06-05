package config

import (
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	RedisURL      string
	APIAddr       string
	ClusterID     string
	AllowedEmails []string
	AdminEmails   []string
	EmailHeader     string
	AIServiceURL    string
	AIServiceTimeout time.Duration
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		DatabaseURL:   getenv("DATABASE_URL", "postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable"),
		RedisURL:      getenv("REDIS_URL", "redis://localhost:6379"),
		APIAddr:       getenv("API_ADDR", ":8080"),
		ClusterID:     getenv("CLUSTER_ID", "local"),
		AllowedEmails: splitAndTrim(os.Getenv("API_ALLOWED_EMAILS")),
		AdminEmails:   splitAndTrim(os.Getenv("API_ADMIN_EMAILS")),
		EmailHeader:        getenv("API_EMAIL_HEADER", "X-Forwarded-Email"),
		AIServiceURL:       os.Getenv("AI_SERVICE_URL"),
		AIServiceTimeout:   parseDuration(getenv("AI_SERVICE_TIMEOUT", "90s"), 90*time.Second),
	}
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func splitAndTrim(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i, p := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(p))
	}
	return parts
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
