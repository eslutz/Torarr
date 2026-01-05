package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/eslutz/torarr/pkg/version"
)

// Event represents a webhook event type
type Event string

const (
	EventCircuitRenewed  Event = "circuit_renewed"
	EventBootstrapFailed Event = "bootstrap_failed"
	EventHealthChanged   Event = "health_changed"
)

// Payload contains the webhook notification data
type Payload struct {
	Event     Event     `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Details   Details   `json:"details"`
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
}

// Details contains event-specific data
type Details struct {
	Bootstrap *int   `json:"bootstrap,omitempty"`
	Circuits  int    `json:"circuits,omitempty"`
	Healthy   bool   `json:"healthy"`
	Error     string `json:"error,omitempty"`
}

// Template represents a webhook template format
type Template string

const (
	TemplateDiscord Template = "discord"
	TemplateSlack   Template = "slack"
	TemplateGotify  Template = "gotify"
	TemplateJSON    Template = "json"
)

// Webhook handles sending webhook notifications
type Webhook struct {
	url      string
	template Template
	timeout  time.Duration
	client   *http.Client
}

// NewWebhook creates a new webhook notifier
func NewWebhook(url string, template Template, timeout time.Duration) *Webhook {
	return &Webhook{
		url:      url,
		template: template,
		timeout:  timeout,
		client:   &http.Client{},
	}
}

// Send sends a webhook notification
func (w *Webhook) Send(ctx context.Context, payload Payload) error {
	if w.url == "" {
		return nil // No webhook configured
	}

	// Add version info
	payload.Version = version.Version
	payload.Commit = version.Commit
	payload.Timestamp = time.Now()

	body, contentType, err := w.formatPayload(payload)
	if err != nil {
		return fmt.Errorf("formatting payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", fmt.Sprintf("Torarr/%s", version.Version))

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// formatPayload formats the payload according to the template
func (w *Webhook) formatPayload(payload Payload) ([]byte, string, error) {
	switch w.template {
	case TemplateDiscord:
		return w.formatDiscord(payload)
	case TemplateSlack:
		return w.formatSlack(payload)
	case TemplateGotify:
		return w.formatGotify(payload)
	case TemplateJSON:
		return w.formatJSON(payload)
	default:
		return w.formatJSON(payload)
	}
}

// formatDiscord formats payload for Discord webhooks
func (w *Webhook) formatDiscord(payload Payload) ([]byte, string, error) {
	color := w.getColor(payload.Event)

	embed := map[string]interface{}{
		"title":       string(payload.Event),
		"description": payload.Message,
		"color":       color,
		"timestamp":   payload.Timestamp.Format(time.RFC3339),
		"footer": map[string]string{
			"text": fmt.Sprintf("Torarr v%s", payload.Version),
		},
		"fields": w.buildFields(payload.Details),
	}

	body := map[string]interface{}{
		"embeds": []interface{}{embed},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling discord payload: %w", err)
	}

	return data, "application/json", nil
}

// formatSlack formats payload for Slack webhooks
func (w *Webhook) formatSlack(payload Payload) ([]byte, string, error) {
	color := w.getColorHex(payload.Event)

	attachment := map[string]interface{}{
		"title":  string(payload.Event),
		"text":   payload.Message,
		"color":  color,
		"footer": fmt.Sprintf("Torarr v%s", payload.Version),
		"ts":     payload.Timestamp.Unix(),
		"fields": w.buildSlackFields(payload.Details),
	}

	body := map[string]interface{}{
		"attachments": []interface{}{attachment},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling slack payload: %w", err)
	}

	return data, "application/json", nil
}

// formatGotify formats payload for Gotify
func (w *Webhook) formatGotify(payload Payload) ([]byte, string, error) {
	priority := w.getPriority(payload.Event)

	body := map[string]interface{}{
		"title":    string(payload.Event),
		"message":  payload.Message,
		"priority": priority,
		"extras": map[string]interface{}{
			"client::display": map[string]interface{}{
				"contentType": "text/markdown",
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling gotify payload: %w", err)
	}

	return data, "application/json", nil
}

// formatJSON formats payload as plain JSON
func (w *Webhook) formatJSON(payload Payload) ([]byte, string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling json payload: %w", err)
	}

	return data, "application/json", nil
}

// getColor returns Discord color code
func (w *Webhook) getColor(event Event) int {
	switch event {
	case EventCircuitRenewed:
		return 3447003 // Blue
	case EventBootstrapFailed:
		return 15158332 // Red
	case EventHealthChanged:
		return 15844367 // Gold
	default:
		return 9807270 // Gray
	}
}

// getColorHex returns Slack color hex
func (w *Webhook) getColorHex(event Event) string {
	switch event {
	case EventCircuitRenewed:
		return "good"
	case EventBootstrapFailed:
		return "danger"
	case EventHealthChanged:
		return "warning"
	default:
		return "#95a5a6"
	}
}

// getPriority returns Gotify priority
func (w *Webhook) getPriority(event Event) int {
	switch event {
	case EventCircuitRenewed:
		return 5
	case EventBootstrapFailed:
		return 8
	case EventHealthChanged:
		return 6
	default:
		return 5
	}
}

// buildFields builds Discord embed fields
func (w *Webhook) buildFields(details Details) []map[string]interface{} {
	fields := []map[string]interface{}{}

	if details.Bootstrap != nil {
		fields = append(fields, map[string]interface{}{
			"name":   "Bootstrap",
			"value":  fmt.Sprintf("%d%%", *details.Bootstrap),
			"inline": true,
		})
	}

	if details.Circuits > 0 {
		fields = append(fields, map[string]interface{}{
			"name":   "Circuits",
			"value":  fmt.Sprintf("%d", details.Circuits),
			"inline": true,
		})
	}

	if details.Error != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Error",
			"value":  details.Error,
			"inline": false,
		})
	}

	return fields
}

// buildSlackFields builds Slack attachment fields
func (w *Webhook) buildSlackFields(details Details) []map[string]interface{} {
	fields := []map[string]interface{}{}

	if details.Bootstrap != nil {
		fields = append(fields, map[string]interface{}{
			"title": "Bootstrap",
			"value": fmt.Sprintf("%d%%", *details.Bootstrap),
			"short": true,
		})
	}

	if details.Circuits > 0 {
		fields = append(fields, map[string]interface{}{
			"title": "Circuits",
			"value": fmt.Sprintf("%d", details.Circuits),
			"short": true,
		})
	}

	if details.Error != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Error",
			"value": details.Error,
			"short": false,
		})
	}

	return fields
}
