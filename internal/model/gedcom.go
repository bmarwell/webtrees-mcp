package model

import (
	"regexp"
	"strings"
)

var (
	nameRE  = regexp.MustCompile(`(?m)^1 NAME ([^/\r\n]*)\s*/([^/\r\n]*)/`)
	birthRE = regexp.MustCompile(`(?ms)^1 BIRT\s*\r?\n2 DATE ([^\r\n]+)`)
	deathRE = regexp.MustCompile(`(?ms)^1 DEAT(?:\s+Y)?\s*\r?\n2 DATE ([^\r\n]+)`)
	sexRE   = regexp.MustCompile(`(?m)^1 SEX ([^\r\n]+)`)
	occupRE = regexp.MustCompile(`(?m)^1 OCCU ([^\r\n]+)`)
	assoRE  = regexp.MustCompile(`(?ms)^1 ASSO @([^@]+)@(?:\r?\n2 RELA ([^\r\n]+))?`)
	husbRE  = regexp.MustCompile(`(?m)^1 HUSB @([^@]+)@`)
	wifeRE  = regexp.MustCompile(`(?m)^1 WIFE @([^@]+)@`)
	chilRE  = regexp.MustCompile(`(?m)^1 CHIL @([^@]+)@`)
)

// ParseIndividualGEDCOM extracts the small subset of GEDCOM metadata useful to
// callers. Unknown GEDCOM tags are intentionally ignored.
func ParseIndividualGEDCOM(id, rawGEDCOM string) PersonDTO {
	person := PersonDTO{ID: id}
	if match := nameRE.FindStringSubmatch(rawGEDCOM); len(match) == 3 {
		person.Name.Given = strings.TrimSpace(match[1])
		person.Name.Surname = strings.TrimSpace(match[2])
	}
	if match := birthRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		person.BirthDate = strings.TrimSpace(match[1])
	}
	if match := deathRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		person.DeathDate = strings.TrimSpace(match[1])
	}
	if match := sexRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		person.Sex = strings.TrimSpace(match[1])
	}
	if match := occupRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		person.Occupation = strings.TrimSpace(match[1])
	}
	for _, match := range assoRE.FindAllStringSubmatch(rawGEDCOM, -1) {
		link := RelativeLink{PersonID: match[1]}
		if len(match) == 3 {
			link.Relationship = strings.TrimSpace(match[2])
		}
		person.Relatives = append(person.Relatives, link)
	}
	return person
}

func ParseFamilyGEDCOM(id, rawGEDCOM string) FamilyDTO {
	family := FamilyDTO{ID: id}
	if match := husbRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		family.Parents = append(family.Parents, RelativeLink{PersonID: match[1], Relationship: "parent"})
	}
	if match := wifeRE.FindStringSubmatch(rawGEDCOM); len(match) == 2 {
		family.Parents = append(family.Parents, RelativeLink{PersonID: match[1], Relationship: "parent"})
	}
	for _, match := range chilRE.FindAllStringSubmatch(rawGEDCOM, -1) {
		family.Children = append(family.Children, RelativeLink{PersonID: match[1], Relationship: "child"})
	}
	return family
}
