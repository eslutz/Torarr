package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPing_Success(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	handler.Ping(w, req)

	res := w.Result()
	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "OK" {
		t.Errorf("expected status 'OK', got '%s'", response["status"])
	}
}

func TestStatusRecorder_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	recorder.WriteHeader(http.StatusNotFound)

	if recorder.status != http.StatusNotFound {
		t.Errorf("expected status to be %d, got %d", http.StatusNotFound, recorder.status)
	}
}

func TestStatusRecorder_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	// Write without setting status explicitly
	if _, err := recorder.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}

	if recorder.status != http.StatusOK {
		t.Errorf("expected default status to be %d, got %d", http.StatusOK, recorder.status)
	}
}

func TestRenew_MethodNotAllowed(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/renew", nil)
	w := httptest.NewRecorder()

	handler.Renew(w, req)

	res := w.Result()
	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, res.StatusCode)
	}
}

func TestSetupRoutes(t *testing.T) {
	mux := http.NewServeMux()

	// Test that we can set up routes without errors
	// We don't call SetupRoutes because it would require a fully initialized handler
	// Instead, test that individual handlers can be called

	handler := &Handler{}

	// Test Ping route (doesn't need Tor client)
	mux.HandleFunc("/ping", handler.Ping)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("ping route not found")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected ping to return %d, got %d", http.StatusOK, w.Code)
	}
}

func TestInstrument_LogsRequest(t *testing.T) {
	handler := &Handler{}

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrapped := handler.instrument("/test", next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrapped(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestInstrument_RecordsStatusCode(t *testing.T) {
	handler := &Handler{}

	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	wrapped := handler.instrument("/test", next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrapped(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
