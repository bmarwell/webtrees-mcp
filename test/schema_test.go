package test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// Set WEBTREES_DSN to run this optional integration check against a real
// Webtrees database. It is skipped during ordinary unit-test runs.
func TestWebtreesSchema(t *testing.T) {
	dsn := os.Getenv("WEBTREES_DSN")
	if dsn == "" {
		t.Skip("WEBTREES_DSN is not set")
	}
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	requiredColumns := map[string][]string{
		"wt_gedcom":      {"gedcom_id", "gedcom_name", "title"},
		"wt_individuals": {"i_id", "i_file", "i_gedcom"},
		"wt_families":    {"f_id", "f_file", "f_gedcom"},
	}
	for table, requiredColumnsForTable := range requiredColumns {
		rows, err := database.Query(`SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
		if err != nil {
			t.Fatal(err)
		}
		columns := map[string]bool{}
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			columns[column] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		for _, required := range requiredColumnsForTable {
			if !columns[required] {
				t.Errorf("%s is missing required column %s", table, required)
			}
		}
	}
}
