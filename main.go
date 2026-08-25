package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mark3labs/mcp-go/server"

	"webtrees-mcp/internal/db"
	webtreesmcp "webtrees-mcp/internal/mcp"
)

func main() {
	dsn := flag.String("dsn", "", "MariaDB connection string")
	prefix := flag.String("prefix", "wt", "Webtrees table prefix")
	httpEnabled := flag.Bool("http", false, "also listen for MCP requests over HTTP")
	httpHost := flag.String("http-host", "127.0.0.1", "HTTP bind address when -http is enabled")
	httpPort := flag.Int("http-port", 8080, "HTTP port when -http is enabled")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("-dsn is required")
	}

	database, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}()
	reader, err := db.NewReader(database, *prefix)
	if err != nil {
		log.Fatal(err)
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
		var httpServer *server.StreamableHTTPServer
		mux := http.NewServeMux()
		mux.Handle("/mcp", accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpServer.ServeHTTP(w, r)
		})))
		transportServer := &http.Server{Addr: address, Handler: mux}
		httpServer = server.NewStreamableHTTPServer(s,
			server.WithStateLess(true),
			server.WithStreamableHTTPServer(transportServer),
		)
		if err := httpServer.Start(address); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
		return
	}
	log.Printf("starting webtrees-mcp on stdio (no network interface or port)")
	if err := server.ServeStdio(s); err != nil {
		log.Printf("server error: %v", err)
	}
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
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

func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		log.Printf("HTTP request method=%s path=%s remote=%s", r.Method, r.URL.RequestURI(), r.RemoteAddr)

		response := &accessLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		log.Printf("HTTP response method=%s path=%s status=%d bytes=%d duration=%s", r.Method, r.URL.RequestURI(), response.status, response.bytes, time.Since(started))
	})
}
