package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandlerDoesNotExposeConnectionDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	statusHandler("ok", http.StatusOK).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %q, want safe status JSON", got)
	}
	if strings.Contains(recorder.Body.String(), "dsn") {
		t.Fatal("health response contains connection details")
	}
}

func TestHealthHandlerReportsNotReady(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	statusHandler("not_ready", http.StatusServiceUnavailable).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"not_ready"}` {
		t.Fatalf("body = %q, want not-ready status JSON", got)
	}
}
