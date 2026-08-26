package test

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"webtrees-mcp/internal/db"
	"webtrees-mcp/internal/genealogy"
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

func TestSearchPersonsUsesIndexedNameCandidates(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	reader, err := db.NewReader(database, "wt")
	if err != nil {
		t.Fatal(err)
	}
	query := regexp.QuoteMeta("SELECT DISTINCT i.i_id, i.i_gedcom FROM wt_individuals i INNER JOIN wt_name n ON n.n_file = i.i_file AND n.n_id = i.i_id WHERE i.i_file = ? AND n.n_surname LIKE ? ORDER BY i.i_id LIMIT ? OFFSET ?")
	rows := sqlmock.NewRows([]string{"i_id", "i_gedcom"}).
		AddRow("I1", "0 @I1@ INDI\n1 NAME Ada /Mayer/").
		AddRow("I2", "0 @I2@ INDI\n1 NAME Bruno /Mayer/")
	mock.ExpectQuery(query).WithArgs("tree", "%Mayer%", 10, 0).WillReturnRows(rows)
	people, err := reader.SearchPersons(genealogy.PersonSearchCriteria{TreeID: "tree", Surname: "Mayer", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || people[0].Person.ID != "I1" || !people[0].Match.DirectHit {
		t.Fatalf("unexpected default search results: %+v", people)
	}
	mock.ExpectQuery(query).WithArgs("tree", "%Mayer%", 10, 0).WillReturnRows(sqlmock.NewRows([]string{"i_id", "i_gedcom"}).
		AddRow("I1", "0 @I1@ INDI\n1 NAME Ada /Mayer/").
		AddRow("I2", "0 @I2@ INDI\n1 NAME Bruno /Mayer/"))
	people, err = reader.SearchPersons(genealogy.PersonSearchCriteria{TreeID: "tree", Surname: "Mayer", IncludeIndirect: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || !people[0].Match.DirectHit || !people[1].Match.DirectHit {
		t.Fatalf("unexpected inclusive search results: %+v", people)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchPersonsAppliesIndexedFiltersBeforeReadingGEDCOM(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	reader, err := db.NewReader(database, "wt")
	if err != nil {
		t.Fatal(err)
	}
	birthMin, birthMax := 1900, 1950
	deathMax := 2020
	query := regexp.QuoteMeta("SELECT DISTINCT i.i_id, i.i_gedcom FROM wt_individuals i INNER JOIN wt_name n ON n.n_file = i.i_file AND n.n_id = i.i_id WHERE i.i_file = ? AND n.n_surname LIKE ? AND n.n_givn LIKE ? AND i.i_sex = ? AND i.i_birth_year >= ? AND i.i_birth_year <= ? AND i.i_death_year <= ? ORDER BY i.i_id LIMIT ? OFFSET ?")
	mock.ExpectQuery(query).WithArgs("tree", "%Mayer%", "%Ada%", "F", birthMin, birthMax, deathMax, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"i_id", "i_gedcom"}).AddRow("I1", "0 @I1@ INDI\n1 NAME Ada /Mayer/"))
	people, err := reader.SearchPersons(genealogy.PersonSearchCriteria{
		TreeID: "tree", Surname: "Mayer", GivenName: "Ada", Sex: "F",
		BirthYearMin: &birthMin, BirthYearMax: &birthMax, DeathYearMax: &deathMax, Limit: 10,
	})
	if err != nil || len(people) != 1 || people[0].Person.ID != "I1" {
		t.Fatalf("unexpected filtered results: %v %+v", err, people)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
