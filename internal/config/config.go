package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TorControlAddress       string
	TorControlPassword      string
	HealthPort              string
	HealthExternalTimeout   int
	HealthExternalEndpoints []string
	LogLevel                string
	WebhookURL              string
	WebhookTemplate         string
	WebhookEvents           []string
	WebhookTimeout          time.Duration
}

func Load() *Config {
	cfg := &Config{
		TorControlAddress:       getEnv("TOR_CONTROL_ADDRESS", "127.0.0.1:9051"),
		TorControlPassword:      os.Getenv("TOR_CONTROL_PASSWORD"),
		HealthPort:              getEnv("HEALTH_PORT", "8085"),
		HealthExternalTimeout:   getEnvAsInt("HEALTH_EXTERNAL_TIMEOUT", 15),
		HealthExternalEndpoints: parseEndpoints(getEnv("HEALTH_EXTERNAL_ENDPOINTS", "")),
		LogLevel:                strings.ToUpper(getEnv("LOG_LEVEL", "INFO")),
		WebhookURL:              getEnv("WEBHOOK_URL", ""),
		WebhookTemplate:         strings.ToLower(getEnv("WEBHOOK_TEMPLATE", "discord")),
		WebhookEvents:           parseEndpoints(getEnv("WEBHOOK_EVENTS", "")),
		WebhookTimeout:          getEnvAsDuration("WEBHOOK_TIMEOUT", 10*time.Second),
	}

	if len(cfg.HealthExternalEndpoints) == 0 {
		cfg.HealthExternalEndpoints = defaultExternalEndpoints()
	}

	if len(cfg.WebhookEvents) == 0 {
		cfg.WebhookEvents = defaultWebhookEvents()
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := strings.TrimSpace(os.Getenv(key))
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		slog.Warn("Invalid configuration value",
			"key", key,
			"value", valueStr,
			"default", defaultValue,
			"error", err,
		)
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := strings.TrimSpace(os.Getenv(key))
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		slog.Warn("Invalid duration configuration value",
			"key", key,
			"value", valueStr,
			"default", defaultValue,
			"error", err,
		)
		return defaultValue
	}
	return value
}

func parseEndpoints(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	var endpoints []string
	for _, part := range parts {
		endpoint := strings.TrimSpace(part)
		if endpoint != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func defaultExternalEndpoints() []string {
	return []string{
		"https://check.torproject.org/api/ip",
	}
}

func defaultWebhookEvents() []string {
	return []string{
		"circuit_renewed",
	}
}
