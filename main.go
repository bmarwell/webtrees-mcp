package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mark3labs/mcp-go/server"

	"webtrees-mcp/internal/db"
	webtreesmcp "webtrees-mcp/internal/mcp"
)

func main() {
	dsn := flag.String("dsn", "", "MariaDB connection string")
	prefix := flag.String("prefix", "wt", "Webtrees table prefix")
	httpEnabled := flag.Bool("http", false, "also listen for MCP requests over HTTP")
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
	log.Printf("starting webtrees-mcp on stdio (no network interface or port)")
	if *httpEnabled {
		address := fmt.Sprintf("127.0.0.1:%d", *httpPort)
		// HTTP clients frequently do not preserve the Mcp-Session-Id header
		// between requests. Stateless mode accepts each request independently
		// and avoids rejecting tools/list or tools/call with a stale/missing ID.
		httpServer := server.NewStreamableHTTPServer(s, server.WithStateLess(true))
		log.Printf("starting webtrees-mcp HTTP transport on http://%s/mcp", address)
		go func() {
			if err := httpServer.Start(address); err != nil {
				log.Printf("HTTP server error: %v", err)
			}
		}()
	}
	if err := server.ServeStdio(s); err != nil {
		log.Printf("server error: %v", err)
	}
}
