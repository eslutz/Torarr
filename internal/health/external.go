package health

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ExternalChecker struct {
	endpoints []string
	timeout   time.Duration
	proxyURL  string
}

type ExternalCheckResult struct {
	Success   bool      `json:"success"`
	IsTor     bool      `json:"is_tor"`
	IP        string    `json:"ip,omitempty"`
	Endpoint  string    `json:"endpoint,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"`
}

func NewExternalChecker(endpoints []string, timeout time.Duration, proxyURL string) *ExternalChecker {
	return &ExternalChecker{
		endpoints: endpoints,
		timeout:   timeout,
		proxyURL:  proxyURL,
	}
}

func (e *ExternalChecker) Check() *ExternalCheckResult {
	return e.performCheck()
}

func (e *ExternalChecker) performCheck() *ExternalCheckResult {
	client := &http.Client{
		Timeout: e.timeout,
	}

	if e.proxyURL != "" {
		proxyURLParsed, err := url.Parse(e.proxyURL)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			}
		}
	}

	for _, endpoint := range e.endpoints {
		result := e.checkEndpoint(client, endpoint)
		if result.Success {
			return result
		}
	}

	return &ExternalCheckResult{
		Success:   false,
		IsTor:     false,
		CheckedAt: time.Now(),
		Error:     "all endpoints failed",
	}
}

func (e *ExternalChecker) checkEndpoint(client *http.Client, endpoint string) *ExternalCheckResult {
	maxRetries := 2
	backoff := 1 * time.Second

	for retry := 0; retry <= maxRetries; retry++ {
		if retry > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}

		result := e.checkEndpointOnce(client, endpoint)
		if result.Success {
			return result
		}
	}

	return &ExternalCheckResult{
		Success:   false,
		IsTor:     false,
		Endpoint:  endpoint,
		CheckedAt: time.Now(),
		Error:     fmt.Sprintf("failed after %d retries", maxRetries),
	}
}

func (e *ExternalChecker) checkEndpointOnce(client *http.Client, endpoint string) *ExternalCheckResult {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return &ExternalCheckResult{
			Success:   false,
			IsTor:     false,
			Endpoint:  endpoint,
			CheckedAt: time.Now(),
			Error:     err.Error(),
		}
	}

	req.Header.Set("User-Agent", "Torarr/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &ExternalCheckResult{
			Success:   false,
			IsTor:     false,
			Endpoint:  endpoint,
			CheckedAt: time.Now(),
			Error:     err.Error(),
		}
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close errors in this context
	}()

	if resp.StatusCode != http.StatusOK {
		return &ExternalCheckResult{
			Success:   false,
			IsTor:     false,
			Endpoint:  endpoint,
			CheckedAt: time.Now(),
			Error:     fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ExternalCheckResult{
			Success:   false,
			IsTor:     false,
			Endpoint:  endpoint,
			CheckedAt: time.Now(),
			Error:     err.Error(),
		}
	}

	isTor, ip := e.parseResponse(endpoint, body)

	return &ExternalCheckResult{
		Success:   true,
		IsTor:     isTor,
		IP:        ip,
		Endpoint:  endpoint,
		CheckedAt: time.Now(),
	}
}

func (e *ExternalChecker) parseResponse(endpoint string, body []byte) (bool, string) {
	bodyStr := string(body)

	if strings.Contains(endpoint, "check.torproject.org") {
		var result struct {
			IsTor bool   `json:"IsTor"`
			IP    string `json:"IP"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			return result.IsTor, result.IP
		}
	}

	if strings.Contains(endpoint, "check.dan.me.uk") {
		isTor := strings.Contains(strings.ToLower(bodyStr), "yes")
		return isTor, ""
	}

	if strings.Contains(endpoint, "ipinfo.io") {
		var result struct {
			IP  string `json:"ip"`
			Org string `json:"org"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			isTor := strings.Contains(strings.ToLower(result.Org), "tor")
			return isTor, result.IP
		}
	}

	return false, ""
}
