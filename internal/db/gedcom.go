package db

import (
	"strconv"
	"strings"

	"webtrees-mcp/internal/domain"
)

type gedcomLine struct {
	level int
	tag   string
	value string
}

func parseGEDCOMLines(raw string) []gedcomLine {
	var lines []gedcomLine
	for _, rawLine := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		fields := strings.SplitN(rawLine, " ", 3)
		if len(fields) < 2 {
			continue
		}
		level, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		line := gedcomLine{level: level, tag: fields[1]}
		if len(fields) == 3 {
			line.value = strings.TrimSpace(fields[2])
		}
		lines = append(lines, line)
	}
	return lines
}

func parseIndividualGEDCOM(id, rawGEDCOM string) domain.Person {
	person := domain.Person{ID: id}
	lines := parseGEDCOMLines(rawGEDCOM)
	var current *domain.Event
	var currentText *[]string
	var currentName *domain.Name
	for _, line := range lines {
		if line.level == 1 {
			current = nil
			currentText = nil
			currentName = nil
			switch line.tag {
			case "NAME":
				person.Names = append(person.Names, parseName(line.value))
				currentName = &person.Names[len(person.Names)-1]
				if len(person.Names) == 1 {
					person.Name = person.Names[0]
				}
			case "_MARNM":
				name := parseName(line.value)
				name.Type = "married"
				person.Names = append(person.Names, name)
				currentName = &person.Names[len(person.Names)-1]
			case "SEX":
				person.Sex = line.value
			case "FAMC":
				person.FamilyLinks = append(person.FamilyLinks, domain.FamilyLink{FamilyID: trimXref(line.value), Role: "child"})
			case "FAMS":
				person.FamilyLinks = append(person.FamilyLinks, domain.FamilyLink{FamilyID: trimXref(line.value), Role: "spouse"})
			case "ASSO":
				person.Relatives = append(person.Relatives, domain.Relative{PersonID: trimXref(line.value)})
			case "NOTE":
				person.Notes = append(person.Notes, line.value)
				currentText = &person.Notes
			case "SOUR":
				person.Sources = append(person.Sources, domain.Source{ID: trimXref(line.value)})
			case "BIRT", "DEAT", "BURI", "CREM", "RESI", "OCCU", "EDUC", "EMIG", "IMMI", "EVEN", "FACT":
				event := domain.Event{Type: strings.ToLower(line.tag), Value: line.value}
				person.Events = append(person.Events, event)
				current = &person.Events[len(person.Events)-1]
				if line.tag == "BIRT" && person.BirthDate == "" && line.value != "" {
					person.BirthDate = line.value
				}
				if line.tag == "DEAT" && person.DeathDate == "" && line.value != "" {
					person.DeathDate = line.value
				}
				if line.tag == "OCCU" && person.Occupation == "" {
					person.Occupation = line.value
				}
			}
			continue
		}
		if line.level == 2 && current != nil {
			switch line.tag {
			case "DATE":
				date := parseDate(line.value)
				current.Date = &date
				if current.Type == "birt" && person.BirthDate == "" {
					person.BirthDate = date.Raw
				}
				if current.Type == "deat" && person.DeathDate == "" {
					person.DeathDate = date.Raw
				}
			case "PLAC":
				current.Place = line.value
			case "NOTE":
				current.Notes = append(current.Notes, line.value)
				currentText = &current.Notes
			case "SOUR":
				current.Sources = append(current.Sources, domain.Source{ID: trimXref(line.value)})
			}
		}
		if line.level == 2 && currentName != nil && line.tag == "TYPE" {
			currentName.Type = strings.ToLower(line.value)
		}
		if line.level >= 2 && line.tag == "RELA" && len(person.Relatives) > 0 {
			person.Relatives[len(person.Relatives)-1].Relationship = line.value
		}
		if line.level >= 2 && (line.tag == "CONT" || line.tag == "CONC") {
			appendContinuation(currentText, line.tag, line.value)
		}
	}
	if len(person.Names) > 0 {
		person.Name = person.Names[0]
	}
	return person
}

func parseFamilyGEDCOM(id, rawGEDCOM string) domain.Family {
	family := domain.Family{ID: id}
	lines := parseGEDCOMLines(rawGEDCOM)
	var current *domain.Event
	var currentText *[]string
	for _, line := range lines {
		if line.level == 1 {
			current = nil
			currentText = nil
			switch line.tag {
			case "HUSB", "WIFE":
				family.Parents = append(family.Parents, domain.Relative{PersonID: trimXref(line.value), Relationship: "parent"})
			case "CHIL":
				family.Children = append(family.Children, domain.Relative{PersonID: trimXref(line.value), Relationship: "child"})
			case "MARR", "DIV", "EVEN", "FACT":
				event := domain.Event{Type: strings.ToLower(line.tag), Value: line.value}
				family.Events = append(family.Events, event)
				current = &family.Events[len(family.Events)-1]
			case "NOTE":
				family.Notes = append(family.Notes, line.value)
				currentText = &family.Notes
			case "SOUR":
				family.Sources = append(family.Sources, domain.Source{ID: trimXref(line.value)})
			}
			continue
		}
		if line.level == 2 && current != nil {
			switch line.tag {
			case "DATE":
				date := parseDate(line.value)
				current.Date = &date
			case "PLAC":
				current.Place = line.value
			case "NOTE":
				current.Notes = append(current.Notes, line.value)
				currentText = &current.Notes
			case "SOUR":
				current.Sources = append(current.Sources, domain.Source{ID: trimXref(line.value)})
			}
		}
		if line.level >= 2 && (line.tag == "CONT" || line.tag == "CONC") {
			appendContinuation(currentText, line.tag, line.value)
		}
	}
	return family
}

func parseName(value string) domain.Name {
	parts := strings.SplitN(value, "/", 3)
	if len(parts) == 3 {
		return domain.Name{Given: strings.TrimSpace(parts[0]), Surname: strings.TrimSpace(parts[1])}
	}
	return domain.Name{Given: strings.TrimSpace(value)}
}

func parseDate(value string) domain.Date {
	value = strings.TrimSpace(value)
	precision := "exact"
	for _, candidate := range []struct{ prefix, name string }{
		{"ABT ", "about"}, {"BEF ", "before"}, {"AFT ", "after"},
		{"CAL ", "calculated"}, {"EST ", "estimated"}, {"INT ", "interpreted"}, {"BET ", "range"},
		{"FROM ", "range"},
	} {
		if strings.HasPrefix(value, candidate.prefix) {
			precision = candidate.name
			break
		}
	}
	return domain.Date{Raw: value, Precision: precision}
}

func trimXref(value string) string {
	return strings.Trim(value, " @")
}

func appendContinuation(target *[]string, tag, value string) {
	if target == nil || len(*target) == 0 {
		return
	}
	if tag == "CONC" {
		(*target)[len(*target)-1] += value
	} else {
		(*target)[len(*target)-1] += "\n" + value
	}
}
