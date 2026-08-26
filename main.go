package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mark3labs/mcp-go/server"

	"webtrees-mcp/internal/db"
	webtreesmcp "webtrees-mcp/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

const (
	databaseStartupTimeout        = 5 * time.Second
	httpShutdownTimeout           = 10 * time.Second
	defaultHTTPDebugBodyLimit     = 4096
	defaultHTTPDebugRedactHeaders = "Authorization,Cookie,Proxy-Authorization,Set-Cookie"
)

func run() error {
	dsn := flag.String("dsn", "", "MariaDB connection string")
	prefix := flag.String("prefix", "wt", "Webtrees table prefix")
	httpEnabled := flag.Bool("http", false, "listen for MCP requests over HTTP")
	httpDebug := flag.Bool("http-debug", false, "log HTTP request and response headers and bodies to stdout")
	httpDebugBodyLimit := flag.Int("http-debug-body-limit", defaultHTTPDebugBodyLimit, "maximum number of request or response body bytes to log")
	httpDebugRedactHeaders := flag.String("http-debug-redact-headers", defaultHTTPDebugRedactHeaders, "comma-separated HTTP headers to redact in debug logs; empty disables header redaction")
	httpHost := flag.String("http-host", "127.0.0.1", "HTTP bind address when -http is enabled")
	httpPort := flag.Int("http-port", 8080, "HTTP port when -http is enabled")
	flag.Parse()
	if *dsn == "" {
		return fmt.Errorf("-dsn is required")
	}
	if *httpDebugBodyLimit < 0 {
		return fmt.Errorf("-http-debug-body-limit must not be negative")
	}
	redactHeaders := parseRedactedHeaders(*httpDebugRedactHeaders)

	database, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}()
	startupContext, cancelStartup := context.WithTimeout(context.Background(), databaseStartupTimeout)
	defer cancelStartup()
	if err := database.PingContext(startupContext); err != nil {
		return fmt.Errorf("database startup check failed: %w", err)
	}
	reader, err := db.NewReader(database, *prefix)
	if err != nil {
		return fmt.Errorf("create database reader: %w", err)
	}
	s := server.NewMCPServer("webtrees-mcp", "0.1.0")
	webtreesmcp.RegisterTools(s, reader)
	if *httpEnabled {
		log.SetOutput(os.Stdout)
		address := net.JoinHostPort(*httpHost, fmt.Sprintf("%d", *httpPort))
		// HTTP clients frequently do not preserve the Mcp-Session-Id header
		// between requests. Stateless mode accepts each request independently
		// and avoids rejecting tools/list or tools/call with a stale/missing ID.
		if *httpHost != "127.0.0.1" && *httpHost != "localhost" && *httpHost != "::1" {
			log.Printf("WARNING: HTTP transport is bound to %s; it has no authentication and may expose genealogy data", *httpHost)
		}
		log.Printf("starting webtrees-mcp HTTP transport on http://%s/mcp", address)
		return serveHTTP(s, address, *httpDebug, *httpDebugBodyLimit, redactHeaders)
	}
	log.Printf("starting webtrees-mcp on stdio (no network interface or port)")
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("stdio server: %w", err)
	}
	return nil
}

func serveHTTP(mcpServer *server.MCPServer, address string, debug bool, bodyLimit int, redactHeaders map[string]struct{}) error {
	var transport *server.StreamableHTTPServer
	mux := http.NewServeMux()
	mux.Handle("/mcp", accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport.ServeHTTP(w, r)
	}), debug, bodyLimit, redactHeaders))
	// The database has already passed the startup check before this handler is
	// installed. The endpoints intentionally expose no connection details.
	mux.Handle("/healthz", accessLog(statusHandler("ok", http.StatusOK), debug, bodyLimit, redactHeaders))
	mux.Handle("/readyz", accessLog(statusHandler("ready", http.StatusOK), debug, bodyLimit, redactHeaders))
	transportServer := &http.Server{Addr: address, Handler: mux}
	transport = server.NewStreamableHTTPServer(mcpServer,
		server.WithStateLess(true),
		server.WithStreamableHTTPServer(transportServer),
	)

	serveErrors := make(chan error, 1)
	go func() { serveErrors <- transport.Start(address) }()

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP server: %w", err)
	case <-signalContext.Done():
		log.Printf("shutdown signal received; stopping HTTP server")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancelShutdown()
	if err := transportServer.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("HTTP graceful shutdown: %w", err)
	}
	if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server after shutdown: %w", err)
	}
	return nil
}

func statusHandler(status string, responseStatus int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := fmt.Sprintf(`{"status":%q}`, status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responseStatus)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, body+"\n")
		}
	})
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	status    int
	bytes     int
	body      []byte
	truncated bool
	bodyLimit int
}

func (w *accessLogResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *accessLogResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *accessLogResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	if len(w.body) < w.bodyLimit {
		remaining := w.bodyLimit - len(w.body)
		if n > remaining {
			w.body = append(w.body, data[:remaining]...)
			w.truncated = true
		} else {
			w.body = append(w.body, data[:n]...)
		}
	} else if n > 0 {
		w.truncated = true
	}
	return n, err
}

func (w *accessLogResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func accessLog(next http.Handler, debug bool, bodyLimit int, redactHeaders map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		log.Printf("HTTP request method=%s path=%s remote=%s", r.Method, r.URL.RequestURI(), r.RemoteAddr)
		var requestBody []byte
		requestTruncated := false
		if debug && r.Body != nil {
			var err error
			requestBody, requestTruncated, r.Body, err = previewBody(r.Body, bodyLimit)
			if err != nil {
				log.Printf("HTTP request body read error: %v", err)
			}
		}
		if debug {
			log.Printf("HTTP request headers=%s body=%s", formatHeaders(r.Header, redactHeaders), formatBody(requestBody, requestTruncated))
		}

		response := &accessLogResponseWriter{ResponseWriter: w, bodyLimit: bodyLimit}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		log.Printf("HTTP response method=%s path=%s status=%d bytes=%d duration=%s", r.Method, r.URL.RequestURI(), response.status, response.bytes, time.Since(started))
		if debug {
			log.Printf("HTTP response headers=%s body=%s", formatHeaders(w.Header(), redactHeaders), formatBody(response.body, response.truncated))
		}
	})
}

func formatHeaders(headers http.Header, redact map[string]struct{}) string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for i, key := range keys {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(key)
		builder.WriteString("=[")
		if _, ok := redact[textproto.CanonicalMIMEHeaderKey(key)]; ok {
			builder.WriteString("[REDACTED]")
		} else {
			builder.WriteString(strings.Join(headers[key], ", "))
		}
		builder.WriteByte(']')
	}
	return builder.String()
}

func parseRedactedHeaders(value string) map[string]struct{} {
	redact := make(map[string]struct{})
	for _, header := range strings.Split(value, ",") {
		header = strings.TrimSpace(header)
		if header != "" {
			redact[textproto.CanonicalMIMEHeaderKey(header)] = struct{}{}
		}
	}
	return redact
}

func previewBody(body io.ReadCloser, limit int) ([]byte, bool, io.ReadCloser, error) {
	read, err := io.ReadAll(io.LimitReader(body, int64(limit)+1))
	truncated := len(read) > limit
	preview := read
	if truncated {
		preview = read[:limit]
	}
	restored := &restoredBody{Reader: io.MultiReader(bytes.NewReader(read), body), closer: body}
	return preview, truncated, restored, err
}

type restoredBody struct {
	io.Reader
	closer io.Closer
}

func (b *restoredBody) Close() error { return b.closer.Close() }

func formatBody(body []byte, truncated bool) string {
	if truncated {
		return fmt.Sprintf("%q...[truncated]", body)
	}
	return fmt.Sprintf("%q", body)
}
