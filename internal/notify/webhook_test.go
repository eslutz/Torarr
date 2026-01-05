package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewWebhook(t *testing.T) {
	url := "https://example.com/webhook"
	template := TemplateDiscord
	timeout := 10 * time.Second

	webhook := NewWebhook(url, template, timeout)

	if webhook.url != url {
		t.Errorf("Expected url %s, got %s", url, webhook.url)
	}
	if webhook.template != template {
		t.Errorf("Expected template %s, got %s", template, webhook.template)
	}
	if webhook.timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, webhook.timeout)
	}
	if webhook.client == nil {
		t.Error("Expected client to be initialized")
	}
}

func TestWebhook_Send_NoURL(t *testing.T) {
	webhook := NewWebhook("", TemplateJSON, 10*time.Second)

	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
	}

	err := webhook.Send(context.Background(), payload)
	if err != nil {
		t.Errorf("Expected no error when URL is empty, got %v", err)
	}
}

func TestWebhook_Send_Success(t *testing.T) {
	tests := []struct {
		name     string
		template Template
	}{
		{"Discord", TemplateDiscord},
		{"Slack", TemplateSlack},
		{"Gotify", TemplateGotify},
		{"JSON", TemplateJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				body, _ := io.ReadAll(r.Body)
				if len(body) == 0 {
					t.Error("Expected non-empty body")
				}

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			webhook := NewWebhook(server.URL, tt.template, 10*time.Second)

			bootstrap := 100
			payload := Payload{
				Event:   EventCircuitRenewed,
				Message: "Circuit renewed successfully",
				Details: Details{
					Bootstrap: &bootstrap,
					Circuits:  3,
				},
			}

			err := webhook.Send(context.Background(), payload)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestWebhook_Send_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	webhook := NewWebhook(server.URL, TemplateJSON, 10*time.Second)

	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
	}

	err := webhook.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}
}

func TestWebhook_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := NewWebhook(server.URL, TemplateJSON, 50*time.Millisecond)

	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
	}

	// Create context with timeout (this is how it's used in production)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := webhook.Send(ctx, payload)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestWebhook_FormatDiscord(t *testing.T) {
	webhook := NewWebhook("https://example.com", TemplateDiscord, 10*time.Second)

	bootstrap := 100
	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
		Details: Details{
			Bootstrap: &bootstrap,
			Circuits:  5,
		},
		Version: "1.0.0",
	}

	body, contentType, err := webhook.formatDiscord(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to unmarshal Discord payload: %v", err)
	}

	embeds, ok := result["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Error("Expected embeds array in Discord payload")
	}

	embed := embeds[0].(map[string]interface{})
	if embed["title"] != string(EventCircuitRenewed) {
		t.Errorf("Expected title %s, got %v", EventCircuitRenewed, embed["title"])
	}
	if embed["description"] != "Test message" {
		t.Errorf("Expected description 'Test message', got %v", embed["description"])
	}

	fields, ok := embed["fields"].([]interface{})
	if !ok {
		t.Error("Expected fields array in embed")
	}
	if len(fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(fields))
	}
}

func TestWebhook_FormatSlack(t *testing.T) {
	webhook := NewWebhook("https://example.com", TemplateSlack, 10*time.Second)

	bootstrap := 75
	payload := Payload{
		Event:   EventBootstrapFailed,
		Message: "Bootstrap failed",
		Details: Details{
			Bootstrap: &bootstrap,
			Error:     "Connection timeout",
		},
		Version: "1.0.0",
	}

	body, contentType, err := webhook.formatSlack(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to unmarshal Slack payload: %v", err)
	}

	attachments, ok := result["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Error("Expected attachments array in Slack payload")
	}

	attachment := attachments[0].(map[string]interface{})
	if attachment["title"] != string(EventBootstrapFailed) {
		t.Errorf("Expected title %s, got %v", EventBootstrapFailed, attachment["title"])
	}
	if attachment["color"] != "danger" {
		t.Errorf("Expected color 'danger', got %v", attachment["color"])
	}
}

func TestWebhook_FormatGotify(t *testing.T) {
	webhook := NewWebhook("https://example.com", TemplateGotify, 10*time.Second)

	payload := Payload{
		Event:   EventHealthChanged,
		Message: "Health status changed",
		Details: Details{
			Healthy: true,
		},
		Version: "1.0.0",
	}

	body, contentType, err := webhook.formatGotify(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to unmarshal Gotify payload: %v", err)
	}

	if result["title"] != string(EventHealthChanged) {
		t.Errorf("Expected title %s, got %v", EventHealthChanged, result["title"])
	}
	if result["message"] != "Health status changed" {
		t.Errorf("Expected message 'Health status changed', got %v", result["message"])
	}

	priority, ok := result["priority"].(float64)
	if !ok || priority != 6 {
		t.Errorf("Expected priority 6, got %v", result["priority"])
	}
}

func TestWebhook_FormatJSON(t *testing.T) {
	webhook := NewWebhook("https://example.com", TemplateJSON, 10*time.Second)

	bootstrap := 100
	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
		Details: Details{
			Bootstrap: &bootstrap,
			Circuits:  3,
		},
		Version: "1.0.0",
		Commit:  "abc123",
	}

	body, contentType, err := webhook.formatJSON(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}

	var result Payload
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON payload: %v", err)
	}

	if result.Event != EventCircuitRenewed {
		t.Errorf("Expected event %s, got %s", EventCircuitRenewed, result.Event)
	}
	if result.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", result.Message)
	}
	if result.Details.Bootstrap == nil || *result.Details.Bootstrap != 100 {
		if result.Details.Bootstrap == nil {
			t.Error("Expected bootstrap 100, got nil")
		} else {
			t.Errorf("Expected bootstrap 100, got %d", *result.Details.Bootstrap)
		}
	}
	if result.Details.Circuits != 3 {
		t.Errorf("Expected circuits 3, got %d", result.Details.Circuits)
	}
}

func TestWebhook_GetColor(t *testing.T) {
	webhook := NewWebhook("", TemplateDiscord, 10*time.Second)

	tests := []struct {
		event Event
		want  int
	}{
		{EventCircuitRenewed, 3447003},
		{EventBootstrapFailed, 15158332},
		{EventHealthChanged, 15844367},
		{Event("unknown"), 9807270},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			got := webhook.getColor(tt.event)
			if got != tt.want {
				t.Errorf("getColor(%s) = %d, want %d", tt.event, got, tt.want)
			}
		})
	}
}

func TestWebhook_GetColorHex(t *testing.T) {
	webhook := NewWebhook("", TemplateSlack, 10*time.Second)

	tests := []struct {
		event Event
		want  string
	}{
		{EventCircuitRenewed, "good"},
		{EventBootstrapFailed, "danger"},
		{EventHealthChanged, "warning"},
		{Event("unknown"), "#95a5a6"},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			got := webhook.getColorHex(tt.event)
			if got != tt.want {
				t.Errorf("getColorHex(%s) = %s, want %s", tt.event, got, tt.want)
			}
		})
	}
}

func TestWebhook_GetPriority(t *testing.T) {
	webhook := NewWebhook("", TemplateGotify, 10*time.Second)

	tests := []struct {
		event Event
		want  int
	}{
		{EventCircuitRenewed, 5},
		{EventBootstrapFailed, 8},
		{EventHealthChanged, 6},
		{Event("unknown"), 5},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			got := webhook.getPriority(tt.event)
			if got != tt.want {
				t.Errorf("getPriority(%s) = %d, want %d", tt.event, got, tt.want)
			}
		})
	}
}

func TestWebhook_BuildFields(t *testing.T) {
	webhook := NewWebhook("", TemplateDiscord, 10*time.Second)

	bootstrap100 := 100
	bootstrap75 := 75
	tests := []struct {
		name    string
		details Details
		want    int
	}{
		{
			name:    "All fields",
			details: Details{Bootstrap: &bootstrap100, Circuits: 5, Error: "test error"},
			want:    3,
		},
		{
			name:    "Bootstrap and Circuits only",
			details: Details{Bootstrap: &bootstrap75, Circuits: 3},
			want:    2,
		},
		{
			name:    "Error only",
			details: Details{Error: "connection failed"},
			want:    1,
		},
		{
			name:    "Empty details",
			details: Details{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := webhook.buildFields(tt.details)
			if len(fields) != tt.want {
				t.Errorf("buildFields() returned %d fields, want %d", len(fields), tt.want)
			}
		})
	}
}

func TestWebhook_BuildSlackFields(t *testing.T) {
	webhook := NewWebhook("", TemplateSlack, 10*time.Second)

	bootstrap := 100
	details := Details{
		Bootstrap: &bootstrap,
		Circuits:  5,
		Error:     "test error",
	}

	fields := webhook.buildSlackFields(details)
	if len(fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(fields))
	}

	// Check first field (Bootstrap)
	if fields[0]["title"] != "Bootstrap" {
		t.Errorf("Expected first field title 'Bootstrap', got %v", fields[0]["title"])
	}
	if fields[0]["value"] != "100%" {
		t.Errorf("Expected first field value '100%%', got %v", fields[0]["value"])
	}
	if fields[0]["short"] != true {
		t.Errorf("Expected first field short=true, got %v", fields[0]["short"])
	}
}

func TestWebhook_FormatPayload_UnknownTemplate(t *testing.T) {
	webhook := NewWebhook("https://example.com", Template("unknown"), 10*time.Second)

	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
	}

	// Should default to JSON
	body, contentType, err := webhook.formatPayload(payload)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("Expected JSON content type for unknown template, got %s", contentType)
	}

	var result Payload
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}
}

func TestWebhook_Send_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := NewWebhook(server.URL, TemplateJSON, 10*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := Payload{
		Event:   EventCircuitRenewed,
		Message: "Test message",
	}

	err := webhook.Send(ctx, payload)
	if err == nil {
		t.Error("Expected error for canceled context, got nil")
	}
}
