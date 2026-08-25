package db

import (
	"database/sql"
	"fmt"
	"regexp"
)

var validPrefix = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Reader owns the database handle used by the read-only repository methods.
type Reader struct {
	db     *sql.DB
	prefix string
}

func NewReader(database *sql.DB, prefix string) (*Reader, error) {
	if database == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if !validPrefix.MatchString(prefix) {
		return nil, fmt.Errorf("invalid table prefix %q", prefix)
	}
	return &Reader{db: database, prefix: prefix}, nil
}
