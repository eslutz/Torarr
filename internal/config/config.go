package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TZ                        string
	TorControlPassword        string
	TorControlAddress         string
	HealthPort                string
	HealthFullTimeout         int
	HealthFullCacheTTL        int
	HealthExternalEndpoints   []string
	LogLevel                  string
}

func Load() *Config {
	return &Config{
		TZ:                 getEnv("TZ", "UTC"),
		TorControlPassword: getEnv("TOR_CONTROL_PASSWORD", ""),
		TorControlAddress:  getEnv("TOR_CONTROL_ADDRESS", "127.0.0.1:9051"),
		HealthPort:         getEnv("HEALTH_PORT", "8080"),
		HealthFullTimeout:  getEnvInt("HEALTH_FULL_TIMEOUT", 15),
		HealthFullCacheTTL: getEnvInt("HEALTH_FULL_CACHE_TTL", 30),
		HealthExternalEndpoints: getEnvSlice("HEALTH_EXTERNAL_ENDPOINTS", []string{
			"https://check.torproject.org/api/ip",
			"https://check.dan.me.uk/",
			"https://ipinfo.io/json",
		}),
		LogLevel: getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
