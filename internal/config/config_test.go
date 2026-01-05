package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any environment variables
	clearEnv()

	cfg := Load()

	if cfg.TorControlAddress != "127.0.0.1:9051" {
		t.Errorf("expected TorControlAddress to be '127.0.0.1:9051', got '%s'", cfg.TorControlAddress)
	}

	if cfg.HealthPort != "8085" {
		t.Errorf("expected HealthPort to be '8085', got '%s'", cfg.HealthPort)
	}

	if cfg.HealthExternalTimeout != 15 {
		t.Errorf("expected HealthExternalTimeout to be 15, got %d", cfg.HealthExternalTimeout)
	}

	if cfg.LogLevel != "INFO" {
		t.Errorf("expected LogLevel to be 'INFO', got '%s'", cfg.LogLevel)
	}

	if len(cfg.HealthExternalEndpoints) == 0 {
		t.Error("expected default external endpoints to be set")
	}

	// Check webhook defaults
	if cfg.WebhookURL != "" {
		t.Errorf("expected WebhookURL to be empty, got '%s'", cfg.WebhookURL)
	}

	if cfg.WebhookTemplate != "discord" {
		t.Errorf("expected WebhookTemplate to be 'discord', got '%s'", cfg.WebhookTemplate)
	}

	if len(cfg.WebhookEvents) == 0 {
		t.Error("expected default webhook events to be set")
	}

	if cfg.WebhookTimeout != 10*time.Second {
		t.Errorf("expected WebhookTimeout to be 10s, got %v", cfg.WebhookTimeout)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()

	if err := os.Setenv("TOR_CONTROL_ADDRESS", "localhost:9999"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TOR_CONTROL_PASSWORD", "secret123"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("HEALTH_PORT", "9000"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("HEALTH_EXTERNAL_TIMEOUT", "30"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("HEALTH_EXTERNAL_ENDPOINTS", "https://example.com/api,https://test.com/check"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("LOG_LEVEL", "debug"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WEBHOOK_URL", "https://hooks.example.com/webhook"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WEBHOOK_TEMPLATE", "slack"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WEBHOOK_EVENTS", "circuit_renewed,bootstrap_failed"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WEBHOOK_TIMEOUT", "30s"); err != nil {
		t.Fatal(err)
	}

	defer clearEnv()

	cfg := Load()

	if cfg.TorControlAddress != "localhost:9999" {
		t.Errorf("expected TorControlAddress to be 'localhost:9999', got '%s'", cfg.TorControlAddress)
	}

	if cfg.TorControlPassword != "secret123" {
		t.Errorf("expected TorControlPassword to be 'secret123', got '%s'", cfg.TorControlPassword)
	}

	if cfg.HealthPort != "9000" {
		t.Errorf("expected HealthPort to be '9000', got '%s'", cfg.HealthPort)
	}

	if cfg.HealthExternalTimeout != 30 {
		t.Errorf("expected HealthExternalTimeout to be 30, got %d", cfg.HealthExternalTimeout)
	}

	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected LogLevel to be 'DEBUG', got '%s'", cfg.LogLevel)
	}

	if len(cfg.HealthExternalEndpoints) != 2 {
		t.Errorf("expected 2 external endpoints, got %d", len(cfg.HealthExternalEndpoints))
	}

	// Check webhook custom values
	if cfg.WebhookURL != "https://hooks.example.com/webhook" {
		t.Errorf("expected WebhookURL to be 'https://hooks.example.com/webhook', got '%s'", cfg.WebhookURL)
	}

	if cfg.WebhookTemplate != "slack" {
		t.Errorf("expected WebhookTemplate to be 'slack', got '%s'", cfg.WebhookTemplate)
	}

	if len(cfg.WebhookEvents) != 2 {
		t.Errorf("expected 2 webhook events, got %d", len(cfg.WebhookEvents))
	}

	if cfg.WebhookTimeout != 30*time.Second {
		t.Errorf("expected WebhookTimeout to be 30s, got %v", cfg.WebhookTimeout)
	}
}

func TestGetEnvAsDuration_ValidValue(t *testing.T) {
	clearEnv()
	if err := os.Setenv("TEST_DURATION", "5m30s"); err != nil {
		t.Fatal(err)
	}
	defer clearEnv()

	result := getEnvAsDuration("TEST_DURATION", 10*time.Second)
	expected := 5*time.Minute + 30*time.Second
	if result != expected {
		t.Errorf("expected duration %v, got %v", expected, result)
	}
}

func TestGetEnvAsDuration_InvalidValue(t *testing.T) {
	clearEnv()
	if err := os.Setenv("TEST_DURATION", "not_a_duration"); err != nil {
		t.Fatal(err)
	}
	defer clearEnv()

	result := getEnvAsDuration("TEST_DURATION", 10*time.Second)
	if result != 10*time.Second {
		t.Errorf("expected default value 10s for invalid input, got %v", result)
	}
}

func TestGetEnvAsDuration_EmptyValue(t *testing.T) {
	clearEnv()
	result := getEnvAsDuration("NONEXISTENT_DURATION", 15*time.Second)
	if result != 15*time.Second {
		t.Errorf("expected default value 15s for empty env var, got %v", result)
	}
}

func TestGetEnvAsInt_InvalidValue(t *testing.T) {
	clearEnv()
	if err := os.Setenv("TEST_INT", "not_a_number"); err != nil {
		t.Fatal(err)
	}
	defer clearEnv()

	result := getEnvAsInt("TEST_INT", 42)
	if result != 42 {
		t.Errorf("expected default value 42 for invalid input, got %d", result)
	}
}

func TestParseEndpoints_EmptyString(t *testing.T) {
	result := parseEndpoints("")
	if result != nil {
		t.Errorf("expected nil for empty string, got %v", result)
	}
}

func TestParseEndpoints_WithSpaces(t *testing.T) {
	result := parseEndpoints(" https://a.com , https://b.com  , https://c.com ")
	if len(result) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(result))
	}

	expected := []string{"https://a.com", "https://b.com", "https://c.com"}
	for i, endpoint := range result {
		if endpoint != expected[i] {
			t.Errorf("expected endpoint %d to be '%s', got '%s'", i, expected[i], endpoint)
		}
	}
}

func TestParseEndpoints_WithEmptyParts(t *testing.T) {
	result := parseEndpoints("https://a.com,,,https://b.com,")
	if len(result) != 2 {
		t.Errorf("expected 2 endpoints (empty parts ignored), got %d", len(result))
	}
}

func TestDefaultExternalEndpoints(t *testing.T) {
	endpoints := defaultExternalEndpoints()
	if len(endpoints) == 0 {
		t.Error("expected at least one default endpoint")
	}

	// Verify it contains the torproject endpoint
	found := false
	for _, ep := range endpoints {
		if ep == "https://check.torproject.org/api/ip" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected default endpoints to include torproject.org check")
	}
}

func TestDefaultWebhookEvents(t *testing.T) {
	events := defaultWebhookEvents()
	if len(events) == 0 {
		t.Error("expected at least one default webhook event")
	}

	// Verify it contains the circuit_renewed event
	found := false
	for _, ev := range events {
		if ev == "circuit_renewed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected default webhook events to include circuit_renewed")
	}
}

func clearEnv() {
	_ = os.Unsetenv("TOR_CONTROL_ADDRESS")
	_ = os.Unsetenv("TOR_CONTROL_PASSWORD")
	_ = os.Unsetenv("HEALTH_PORT")
	_ = os.Unsetenv("HEALTH_EXTERNAL_TIMEOUT")
	_ = os.Unsetenv("HEALTH_EXTERNAL_ENDPOINTS")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("WEBHOOK_URL")
	_ = os.Unsetenv("WEBHOOK_TEMPLATE")
	_ = os.Unsetenv("WEBHOOK_EVENTS")
	_ = os.Unsetenv("WEBHOOK_TIMEOUT")
	_ = os.Unsetenv("TEST_INT")
	_ = os.Unsetenv("TEST_DURATION")
}
