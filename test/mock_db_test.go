package test

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"webtrees-mcp/internal/db"
)

func TestGetPersonParsesGEDCOM(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	reader, err := db.NewReader(database, "wt")
	if err != nil {
		t.Fatal(err)
	}
	query := regexp.QuoteMeta("SELECT i_gedcom FROM wt_individuals WHERE i_file = ? AND i_id = ?")
	mock.ExpectQuery(query).
		WithArgs("tree", "I1").
		WillReturnRows(sqlmock.NewRows([]string{"i_gedcom"}).AddRow("0 @I1@ INDI\n1 NAME Ada /Lovelace/\n1 BIRT\n2 DATE 10 DEC 1815"))

	person, err := reader.GetPerson("tree", "I1")
	if err != nil {
		t.Fatal(err)
	}
	if person.ID != "I1" || person.Name.Given != "Ada" || person.Name.Surname != "Lovelace" || person.BirthDate != "10 DEC 1815" {
		t.Fatalf("unexpected person: %+v", person)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNewReaderRejectsUnsafePrefix(t *testing.T) {
	if _, err := db.NewReader(nil, "wt; DROP TABLE users"); err == nil {
		t.Fatal("expected unsafe prefix to be rejected")
	}
}
