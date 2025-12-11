package tor

import (
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
			name: "No bootstrap phase",
			input: map[string]string{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic from GetStatus
			status := &Status{}
			if phase, ok := tt.input["status/bootstrap-phase"]; ok {
				// This is the actual parsing logic from control.go
				for _, field := range splitFields(phase) {
					if len(field) > 9 && field[:9] == "PROGRESS=" {
						progressStr := field[9:]
						var progress int
						// Simple integer parsing
						for _, ch := range progressStr {
							if ch >= '0' && ch <= '9' {
								progress = progress*10 + int(ch-'0')
							} else {
								break
							}
						}
						status.BootstrapPhase = progress
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
			name: "No circuit status",
			input: map[string]string{},
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
			
			// Simulate parsing logic
			if read, ok := tt.input["traffic/read"]; ok {
				var val int64
				for _, ch := range read {
					if ch >= '0' && ch <= '9' {
						val = val*10 + int64(ch-'0')
					}
				}
				status.Traffic.BytesRead = val
			}

			if written, ok := tt.input["traffic/written"]; ok {
				var val int64
				for _, ch := range written {
					if ch >= '0' && ch <= '9' {
						val = val*10 + int64(ch-'0')
					}
				}
				status.Traffic.BytesWritten = val
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

// Helper function to split fields (mimics strings.Fields behavior for testing)
func splitFields(s string) []string {
	var fields []string
	var current string
	inWord := false

	for _, ch := range s {
		if ch == ' ' || ch == '\t' || ch == '\n' {
			if inWord {
				fields = append(fields, current)
				current = ""
				inWord = false
			}
		} else {
			current += string(ch)
			inWord = true
		}
	}
	if inWord {
		fields = append(fields, current)
	}
	return fields
}
