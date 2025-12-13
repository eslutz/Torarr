package health

import (
	"testing"
	"time"
)

func TestNewExternalChecker(t *testing.T) {
	endpoints := []string{"https://example.com"}
	timeout := 10 * time.Second
	proxyURL := "socks5://127.0.0.1:9050"

	checker := NewExternalChecker(endpoints, timeout, proxyURL)

	if checker == nil {
		t.Fatal("expected checker to be created, got nil")
	}

	if len(checker.endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(checker.endpoints))
	}

	if checker.timeout != timeout {
		t.Errorf("expected timeout to be %v, got %v", timeout, checker.timeout)
	}

	if checker.proxyURL != proxyURL {
		t.Errorf("expected proxyURL to be '%s', got '%s'", proxyURL, checker.proxyURL)
	}
}

func TestParseResponse_TorProject(t *testing.T) {
	checker := NewExternalChecker(nil, 0, "")

	tests := []struct {
		name        string
		endpoint    string
		body        []byte
		expectedTor bool
		expectedIP  string
	}{
		{
			name:        "TorProject - is Tor",
			endpoint:    "https://check.torproject.org/api/ip",
			body:        []byte(`{"IsTor":true,"IP":"185.220.101.1"}`),
			expectedTor: true,
			expectedIP:  "185.220.101.1",
		},
		{
			name:        "TorProject - not Tor",
			endpoint:    "https://check.torproject.org/api/ip",
			body:        []byte(`{"IsTor":false,"IP":"1.2.3.4"}`),
			expectedTor: false,
			expectedIP:  "1.2.3.4",
		},
		{
			name:        "TorProject - invalid JSON",
			endpoint:    "https://check.torproject.org/api/ip",
			body:        []byte(`invalid json`),
			expectedTor: false,
			expectedIP:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTor, ip := checker.parseResponse(tt.endpoint, tt.body)

			if isTor != tt.expectedTor {
				t.Errorf("expected isTor to be %v, got %v", tt.expectedTor, isTor)
			}

			if ip != tt.expectedIP {
				t.Errorf("expected IP to be '%s', got '%s'", tt.expectedIP, ip)
			}
		})
	}
}

func TestParseResponse_DanMeUk(t *testing.T) {
	checker := NewExternalChecker(nil, 0, "")

	tests := []struct {
		name        string
		endpoint    string
		body        []byte
		expectedTor bool
	}{
		{
			name:        "Dan.me.uk - Yes",
			endpoint:    "https://check.dan.me.uk",
			body:        []byte("Yes"),
			expectedTor: true,
		},
		{
			name:        "Dan.me.uk - yes (lowercase)",
			endpoint:    "https://check.dan.me.uk",
			body:        []byte("yes"),
			expectedTor: true,
		},
		{
			name:        "Dan.me.uk - No",
			endpoint:    "https://check.dan.me.uk",
			body:        []byte("No"),
			expectedTor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTor, _ := checker.parseResponse(tt.endpoint, tt.body)

			if isTor != tt.expectedTor {
				t.Errorf("expected isTor to be %v, got %v", tt.expectedTor, isTor)
			}
		})
	}
}

func TestParseResponse_IPInfo(t *testing.T) {
	checker := NewExternalChecker(nil, 0, "")

	tests := []struct {
		name        string
		endpoint    string
		body        []byte
		expectedTor bool
		expectedIP  string
	}{
		{
			name:        "IPInfo - Tor org",
			endpoint:    "https://ipinfo.io/json",
			body:        []byte(`{"ip":"185.220.101.1","org":"AS12345 TOR Network"}`),
			expectedTor: true,
			expectedIP:  "185.220.101.1",
		},
		{
			name:        "IPInfo - Regular org",
			endpoint:    "https://ipinfo.io/json",
			body:        []byte(`{"ip":"1.2.3.4","org":"AS54321 Regular ISP"}`),
			expectedTor: false,
			expectedIP:  "1.2.3.4",
		},
		{
			name:        "IPInfo - Tor lowercase",
			endpoint:    "https://ipinfo.io/json",
			body:        []byte(`{"ip":"185.220.101.1","org":"tor exit node"}`),
			expectedTor: true,
			expectedIP:  "185.220.101.1",
		},
		{
			name:        "IPInfo - Invalid JSON",
			endpoint:    "https://ipinfo.io/json",
			body:        []byte(`invalid`),
			expectedTor: false,
			expectedIP:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTor, ip := checker.parseResponse(tt.endpoint, tt.body)

			if isTor != tt.expectedTor {
				t.Errorf("expected isTor to be %v, got %v", tt.expectedTor, isTor)
			}

			if ip != tt.expectedIP {
				t.Errorf("expected IP to be '%s', got '%s'", tt.expectedIP, ip)
			}
		})
	}
}

func TestParseResponse_UnknownEndpoint(t *testing.T) {
	checker := NewExternalChecker(nil, 0, "")

	isTor, ip := checker.parseResponse("https://unknown.com", []byte("anything"))

	if isTor {
		t.Error("expected isTor to be false for unknown endpoint")
	}

	if ip != "" {
		t.Errorf("expected IP to be empty for unknown endpoint, got '%s'", ip)
	}
}

func TestExternalCheckResult_Structure(t *testing.T) {
	result := &ExternalCheckResult{
		Success:   true,
		IsTor:     true,
		IP:        "185.220.101.1",
		Endpoint:  "https://check.torproject.org/api/ip",
		CheckedAt: time.Now(),
		Error:     "",
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}

	if !result.IsTor {
		t.Error("expected IsTor to be true")
	}

	if result.IP != "185.220.101.1" {
		t.Errorf("expected IP to be '185.220.101.1', got '%s'", result.IP)
	}

	if result.Endpoint != "https://check.torproject.org/api/ip" {
		t.Errorf("expected Endpoint to be 'https://check.torproject.org/api/ip', got '%s'", result.Endpoint)
	}

	if result.Error != "" {
		t.Errorf("expected Error to be empty, got '%s'", result.Error)
	}
}

func TestExternalCheckResult_WithError(t *testing.T) {
	result := &ExternalCheckResult{
		Success:   false,
		IsTor:     false,
		CheckedAt: time.Now(),
		Error:     "connection timeout",
	}

	if result.Success {
		t.Error("expected Success to be false")
	}

	if result.IsTor {
		t.Error("expected IsTor to be false")
	}

	if result.Error != "connection timeout" {
		t.Errorf("expected Error to be 'connection timeout', got '%s'", result.Error)
	}
}

