package db

import "testing"

func TestParseIndividualGEDCOMPreservesEventsAndRelationships(t *testing.T) {
	raw := "0 @I44@ INDI\n" +
		"1 NAME Ada /Mayer/\n" +
		"1 SEX F\n" +
		"1 FAMC @F1@\n" +
		"1 FAMS @F2@\n" +
		"1 ASSO @I45@\n" +
		"2 RELA spouse\n" +
		"1 BIRT\n" +
		"2 DATE ABT 1982\n" +
		"2 PLAC Stadthagen\n" +
		"2 NOTE Born near the old station\n" +
		"3 CONT Second line\n" +
		"1 OCCU Carpenter\n" +
		"1 OCCU Teacher\n" +
		"1 DEAT\n" +
		"2 DATE BET 2025 AND 2026\n"

	person := parseIndividualGEDCOM("I44", raw)
	if person.BirthDate != "ABT 1982" || person.DeathDate != "BET 2025 AND 2026" || person.Occupation != "Carpenter" {
		t.Fatalf("legacy fields were not preserved: %+v", person)
	}
	if len(person.Events) != 4 {
		t.Fatalf("expected four events, got %d: %+v", len(person.Events), person.Events)
	}
	if person.Events[0].Date == nil || person.Events[0].Date.Precision != "about" || person.Events[0].Place != "Stadthagen" {
		t.Fatalf("birth event lost date precision or place: %+v", person.Events[0])
	}
	if len(person.Events[0].Notes) != 1 || person.Events[0].Notes[0] != "Born near the old station\nSecond line" {
		t.Fatalf("note continuation was not preserved: %+v", person.Events[0].Notes)
	}
	if len(person.Relatives) != 1 || person.Relatives[0].Relationship != "spouse" {
		t.Fatalf("association relationship was not preserved: %+v", person.Relatives)
	}
	if len(person.FamilyLinks) != 2 || person.FamilyLinks[0].Role != "child" || person.FamilyLinks[1].Role != "spouse" {
		t.Fatalf("family links were not preserved: %+v", person.FamilyLinks)
	}
}

func TestParseFamilyGEDCOMPreservesEventsNotesAndSources(t *testing.T) {
	raw := "0 @F2@ FAM\n" +
		"1 HUSB @I44@\n" +
		"1 WIFE @I45@\n" +
		"1 CHIL @I46@\n" +
		"1 MARR\n" +
		"2 DATE 2005\n" +
		"2 PLAC Los Angeles\n" +
		"2 SOUR @S1@\n" +
		"2 NOTE Civil record\n" +
		"3 CONC reference"

	family := parseFamilyGEDCOM("F2", raw)
	if len(family.Parents) != 2 || len(family.Children) != 1 {
		t.Fatalf("family links were not parsed: %+v", family)
	}
	if len(family.Events) != 1 || family.Events[0].Date == nil || family.Events[0].Place != "Los Angeles" {
		t.Fatalf("family event was not parsed: %+v", family.Events)
	}
	if len(family.Events[0].Sources) != 1 || family.Events[0].Sources[0].ID != "S1" {
		t.Fatalf("family source was not parsed: %+v", family.Events[0].Sources)
	}
	if len(family.Events[0].Notes) != 1 || family.Events[0].Notes[0] != "Civil recordreference" {
		t.Fatalf("family note continuation was not parsed: %+v", family.Events[0].Notes)
	}
}
