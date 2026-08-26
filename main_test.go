package main

import (
	"io"
	"log"
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

func TestPreviewBodyBoundsLogAndPreservesRequest(t *testing.T) {
	original := io.NopCloser(strings.NewReader("0123456789"))
	preview, truncated, restored, err := previewBody(original, 4)
	if err != nil {
		t.Fatalf("previewBody() error = %v", err)
	}
	if !truncated || string(preview) != "0123" {
		t.Fatalf("preview = %q, truncated = %v; want %q, true", preview, truncated, "0123")
	}
	full, err := io.ReadAll(restored)
	if err != nil {
		t.Fatalf("reading restored body: %v", err)
	}
	if string(full) != "0123456789" {
		t.Fatalf("restored body = %q, want original body", full)
	}
}

func TestFormatHeadersRedactsConfiguredHeaders(t *testing.T) {
	headers := http.Header{"Authorization": {"Bearer secret"}, "X-Test": {"visible"}}
	got := formatHeaders(headers, parseRedactedHeaders("authorization"))
	if strings.Contains(got, "secret") || !strings.Contains(got, "[REDACTED]") || !strings.Contains(got, "visible") {
		t.Fatalf("formatted headers = %q; expected redaction and visible headers", got)
	}
}

func TestAccessLogResponseBodyIsBounded(t *testing.T) {
	var responseBody strings.Builder
	handler := accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "0123456789")
	}), true, 4, nil)
	logWriter := log.Writer()
	defer log.SetOutput(logWriter)
	log.SetOutput(&responseBody)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(responseBody.String(), `body="0123"...[truncated]`) {
		t.Fatalf("debug log = %q; expected bounded response body", responseBody.String())
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
