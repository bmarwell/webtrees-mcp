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

func TestSearchPersonsExcludesIndirectHitsByDefault(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	reader, err := db.NewReader(database, "wt")
	if err != nil {
		t.Fatal(err)
	}
	query := regexp.QuoteMeta("SELECT i_id, i_gedcom FROM wt_individuals WHERE i_file = ? AND i_gedcom LIKE ? ORDER BY i_id")
	rows := sqlmock.NewRows([]string{"i_id", "i_gedcom"}).
		AddRow("I1", "0 @I1@ INDI\n1 NAME Ada /Mayer/").
		AddRow("I2", "0 @I2@ INDI\n1 NAME Bruno /Schmidt/\n1 OCCU Mayer")
	mock.ExpectQuery(query).WithArgs("tree", "%Mayer%").WillReturnRows(rows)
	people, err := reader.SearchPersons("tree", "Mayer", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].Person.ID != "I1" || !people[0].Match.DirectHit {
		t.Fatalf("unexpected default search results: %+v", people)
	}
	mock.ExpectQuery(query).WithArgs("tree", "%Mayer%").WillReturnRows(sqlmock.NewRows([]string{"i_id", "i_gedcom"}).
		AddRow("I1", "0 @I1@ INDI\n1 NAME Ada /Mayer/").
		AddRow("I2", "0 @I2@ INDI\n1 NAME Bruno /Schmidt/\n1 OCCU Mayer"))
	people, err = reader.SearchPersons("tree", "Mayer", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || !people[0].Match.DirectHit || people[1].Match.DirectHit {
		t.Fatalf("unexpected inclusive search results: %+v", people)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
