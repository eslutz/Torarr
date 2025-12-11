package config

import (
	"os"
	"testing"
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
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()

	os.Setenv("TOR_CONTROL_ADDRESS", "localhost:9999")
	os.Setenv("TOR_CONTROL_PASSWORD", "secret123")
	os.Setenv("HEALTH_PORT", "9000")
	os.Setenv("HEALTH_EXTERNAL_TIMEOUT", "30")
	os.Setenv("HEALTH_EXTERNAL_ENDPOINTS", "https://example.com/api,https://test.com/check")
	os.Setenv("LOG_LEVEL", "debug")

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
}

func TestGetEnvAsInt_InvalidValue(t *testing.T) {
	clearEnv()
	os.Setenv("TEST_INT", "not_a_number")
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

func clearEnv() {
	os.Unsetenv("TOR_CONTROL_ADDRESS")
	os.Unsetenv("TOR_CONTROL_PASSWORD")
	os.Unsetenv("HEALTH_PORT")
	os.Unsetenv("HEALTH_EXTERNAL_TIMEOUT")
	os.Unsetenv("HEALTH_EXTERNAL_ENDPOINTS")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("TEST_INT")
}
