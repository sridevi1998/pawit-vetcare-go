package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment       string
	Port              string
	DatabaseURL       string
	AllowedOrigins    []string
	TrustedProxyCIDRs []string
	JWTSigningKey     string
	AllowDevAuth      bool
	RateLimitRPM      int
	RequestBodyLimit  int64
}

func Load() (Config, error) {
	cfg := Config{
		Environment:       get("PAWIT_ENV", "development"),
		Port:              get("PORT", "8080"),
		DatabaseURL:       os.Getenv("PAWIT_DATABASE_URL"),
		AllowedOrigins:    csv(get("PAWIT_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		TrustedProxyCIDRs: csv(os.Getenv("PAWIT_TRUSTED_PROXY_CIDRS")),
		JWTSigningKey:     os.Getenv("PAWIT_JWT_SIGNING_KEY"),
		AllowDevAuth:      boolEnv("PAWIT_ALLOW_DEV_AUTH", true),
		RateLimitRPM:      intEnv("PAWIT_RATE_LIMIT_RPM", 120),
		RequestBodyLimit:  int64(intEnv("PAWIT_REQUEST_BODY_LIMIT_BYTES", 1<<20)),
	}

	if cfg.Environment == "production" {
		if cfg.JWTSigningKey == "" {
			return Config{}, errors.New("PAWIT_JWT_SIGNING_KEY is required in production")
		}
		if cfg.AllowDevAuth {
			return Config{}, errors.New("PAWIT_ALLOW_DEV_AUTH must be false in production")
		}
		if cfg.DatabaseURL == "" {
			return Config{}, errors.New("PAWIT_DATABASE_URL is required in production")
		}
	}

	if cfg.Port == "" {
		return Config{}, errors.New("PORT cannot be empty")
	}
	if cfg.RateLimitRPM < 1 {
		return Config{}, fmt.Errorf("PAWIT_RATE_LIMIT_RPM must be positive, got %d", cfg.RateLimitRPM)
	}
	if cfg.RequestBodyLimit < 1024 {
		return Config{}, fmt.Errorf("PAWIT_REQUEST_BODY_LIMIT_BYTES must be at least 1024, got %d", cfg.RequestBodyLimit)
	}
	for _, value := range cfg.TrustedProxyCIDRs {
		if net.ParseIP(value) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(value); err != nil {
			return Config{}, fmt.Errorf("PAWIT_TRUSTED_PROXY_CIDRS contains invalid IP or CIDR %q", value)
		}
	}

	return cfg, nil
}

func (c Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c Config) RateWindow() time.Duration {
	return time.Minute
}

func get(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csv(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
