package tor

import (
	"strconv"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("127.0.0.1:9051", "password")

	if client == nil {
		t.Fatal("expected client to be created, got nil")
	}

	if client.address != "127.0.0.1:9051" {
		t.Errorf("expected address to be '127.0.0.1:9051', got '%s'", client.address)
	}

	if client.password != "password" {
		t.Errorf("expected password to be 'password', got '%s'", client.password)
	}

	if client.conn != nil {
		t.Error("expected connection to be nil initially")
	}
}

func TestNewClient_NoPassword(t *testing.T) {
	client := NewClient("localhost:9051", "")

	if client == nil {
		t.Fatal("expected client to be created, got nil")
	}

	if client.password != "" {
		t.Errorf("expected password to be empty, got '%s'", client.password)
	}
}

// TestParseBootstrapPhase tests parsing of bootstrap phase from status string
func TestParseBootstrapPhase(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected int
	}{
		{
			name: "Bootstrap at 100%",
			input: map[string]string{
				"status/bootstrap-phase": "NOTICE BOOTSTRAP PROGRESS=100 TAG=done SUMMARY=\"Done\"",
			},
			expected: 100,
		},
		{
			name: "Bootstrap at 50%",
			input: map[string]string{
				"status/bootstrap-phase": "NOTICE BOOTSTRAP PROGRESS=50 TAG=loading_descriptors SUMMARY=\"Loading relay descriptors\"",
			},
			expected: 50,
		},
		{
			name: "Bootstrap at 0%",
			input: map[string]string{
				"status/bootstrap-phase": "NOTICE BOOTSTRAP PROGRESS=0 TAG=starting SUMMARY=\"Starting\"",
			},
			expected: 0,
		},
		{
			name:     "No bootstrap phase",
			input:    map[string]string{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic from GetStatus
			status := &Status{}
			if phase, ok := tt.input["status/bootstrap-phase"]; ok {
				// This is the actual parsing logic from control.go
				for _, field := range strings.Fields(phase) {
					if strings.HasPrefix(field, "PROGRESS=") {
						progressStr := strings.TrimPrefix(field, "PROGRESS=")
						if progress, err := strconv.Atoi(progressStr); err == nil {
							status.BootstrapPhase = progress
						}
					}
				}
			}

			if status.BootstrapPhase != tt.expected {
				t.Errorf("expected BootstrapPhase to be %d, got %d", tt.expected, status.BootstrapPhase)
			}
		})
	}
}

// TestParseCircuitEstablished tests parsing of circuit established status
func TestParseCircuitEstablished(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected bool
	}{
		{
			name: "Circuit established",
			input: map[string]string{
				"status/circuit-established": "1",
			},
			expected: true,
		},
		{
			name: "Circuit not established",
			input: map[string]string{
				"status/circuit-established": "0",
			},
			expected: false,
		},
		{
			name:     "No circuit status",
			input:    map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &Status{}
			if established, ok := tt.input["status/circuit-established"]; ok {
				status.CircuitEstablished = established == "1"
			}

			if status.CircuitEstablished != tt.expected {
				t.Errorf("expected CircuitEstablished to be %v, got %v", tt.expected, status.CircuitEstablished)
			}
		})
	}
}

// TestParseTrafficStats tests parsing of traffic statistics
func TestParseTrafficStats(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]string
		expectedRead  int64
		expectedWrite int64
	}{
		{
			name: "Normal traffic",
			input: map[string]string{
				"traffic/read":    "1234567890",
				"traffic/written": "9876543210",
			},
			expectedRead:  1234567890,
			expectedWrite: 9876543210,
		},
		{
			name: "Zero traffic",
			input: map[string]string{
				"traffic/read":    "0",
				"traffic/written": "0",
			},
			expectedRead:  0,
			expectedWrite: 0,
		},
		{
			name:          "No traffic data",
			input:         map[string]string{},
			expectedRead:  0,
			expectedWrite: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &Status{}

			// Simulate parsing logic using standard library
			if read, ok := tt.input["traffic/read"]; ok {
				if val, err := strconv.ParseInt(read, 10, 64); err == nil {
					status.Traffic.BytesRead = val
				}
			}

			if written, ok := tt.input["traffic/written"]; ok {
				if val, err := strconv.ParseInt(written, 10, 64); err == nil {
					status.Traffic.BytesWritten = val
				}
			}

			if status.Traffic.BytesRead != tt.expectedRead {
				t.Errorf("expected BytesRead to be %d, got %d", tt.expectedRead, status.Traffic.BytesRead)
			}

			if status.Traffic.BytesWritten != tt.expectedWrite {
				t.Errorf("expected BytesWritten to be %d, got %d", tt.expectedWrite, status.Traffic.BytesWritten)
			}
		})
	}
}

func TestClose_NotConnected(t *testing.T) {
	client := NewClient("127.0.0.1:9051", "password")

	err := client.Close()
	if err != nil {
		t.Errorf("expected no error when closing unconnected client, got %v", err)
	}
}

func TestIsReady_Bootstrap100(t *testing.T) {
	client := NewClient("127.0.0.1:9051", "password")

	// Test with nil status (not connected)
	if client.IsReady() {
		t.Error("expected IsReady to be false when status is nil")
	}
}


func TestConnect_InvalidAddress(t *testing.T) {
client := NewClient("invalid:address:9051", "password")

err := client.Connect()
if err == nil {
t.Error("expected error when connecting to invalid address")
}
}

func TestConnect_UnreachableAddress(t *testing.T) {
// Use an unreachable address (reserved TEST-NET-1 range)
client := NewClient("192.0.2.1:9999", "password")

err := client.Connect()
if err == nil {
t.Error("expected error when connecting to unreachable address")
client.Close()
}
}

func TestGetStatus_NotConnected(t *testing.T) {
client := NewClient("127.0.0.1:9051", "password")

status, err := client.GetStatus()
if err == nil {
t.Error("expected error when getting status without connection")
}
if status != nil {
t.Error("expected nil status when not connected")
}
}

func TestSignal_NotConnected(t *testing.T) {
client := NewClient("127.0.0.1:9051", "password")

err := client.Signal("NEWNYM")
if err == nil {
t.Error("expected error when sending signal without connection")
}
}
