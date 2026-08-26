package db

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"webtrees-mcp/internal/domain"
)

// GetPerson reads one individual. The table name is validated when Reader is
// constructed; values remain regular SQL parameters.
func (r *Reader) GetPerson(treeID, xref string) (*domain.Person, error) {
	query := fmt.Sprintf("SELECT i_gedcom FROM %s_individuals WHERE i_file = ? AND i_id = ?", r.prefix)
	var raw string
	if err := r.db.QueryRow(query, treeID, xref).Scan(&raw); err != nil {
		return nil, err
	}
	person := parseIndividualGEDCOM(xref, raw)
	return &person, nil
}

func (r *Reader) SearchPersons(treeID, surname string) ([]domain.PersonSearchResult, error) {
	query := fmt.Sprintf("SELECT i_id, i_gedcom FROM %s_individuals WHERE i_file = ? AND i_gedcom LIKE ? ORDER BY i_id", r.prefix)
	rows, err := r.db.Query(query, treeID, "%"+surname+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var people []domain.PersonSearchResult
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		person := parseIndividualGEDCOM(id, raw)
		people = append(people, domain.PersonSearchResult{Person: person, Match: searchMatch(person, surname)})
	}
	return people, rows.Err()
}

func searchMatch(person domain.Person, query string) domain.SearchMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	names := person.Names
	if len(names) == 0 && (person.Name.Given != "" || person.Name.Surname != "") {
		names = []domain.Name{person.Name}
	}
	for index, nameValue := range names {
		name := strings.ToLower(nameValue.Given + " " + nameValue.Surname)
		if query != "" && strings.Contains(name, query) {
			field := "name"
			if index > 0 {
				field = nameSearchField(nameValue.Type)
			}
			return domain.SearchMatch{DirectHit: true, Fields: []string{field}}
		}
	}
	return domain.SearchMatch{Fields: []string{"gedcom_record"}}
}

func nameSearchField(nameType string) string {
	switch strings.ToLower(nameType) {
	case "birth", "maiden":
		return "birth_name"
	case "married":
		return "married_name"
	default:
		return "alternate_name"
	}
}

func (r *Reader) GetFamily(treeID, xref string) (*domain.Family, error) {
	query := fmt.Sprintf("SELECT f_gedcom FROM %s_families WHERE f_file = ? AND f_id = ?", r.prefix)
	var raw string
	if err := r.db.QueryRow(query, treeID, xref).Scan(&raw); err != nil {
		return nil, err
	}
	family := parseFamilyGEDCOM(xref, raw)
	return &family, nil
}

func (r *Reader) ListTrees() ([]domain.Tree, error) {
	query := fmt.Sprintf("SELECT gedcom_id, gedcom_name, title FROM %s_gedcom ORDER BY gedcom_id", r.prefix)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trees []domain.Tree
	type treeRow struct {
		id    string
		name  string
		title string
	}
	for rows.Next() {
		var row treeRow
		if err := rows.Scan(&row.id, &row.name, &row.title); err != nil {
			return nil, err
		}
		trees = append(trees, domain.Tree{ID: row.id, Name: row.name, Title: row.title})
	}
	return trees, rows.Err()
}

func (r *Reader) ListRecentlyBorn(treeID string, limit int) ([]domain.Person, error) {
	return r.listByEvent(treeID, limit, true)
}

func (r *Reader) ListRecentlyDeceased(treeID string, limit int) ([]domain.Person, error) {
	return r.listByEvent(treeID, limit, false)
}

func (r *Reader) listByEvent(treeID string, limit int, born bool) ([]domain.Person, error) {
	if limit < 1 {
		limit = 10
	}
	query := fmt.Sprintf("SELECT i_id, i_gedcom FROM %s_individuals WHERE i_file = ?", r.prefix)
	rows, err := r.db.Query(query, treeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type candidate struct {
		person domain.Person
		year   int
	}
	var candidates []candidate
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		person := parseIndividualGEDCOM(id, raw)
		date := person.BirthDate
		if !born {
			date = person.DeathDate
		}
		if date == "" {
			continue
		}
		candidates = append(candidates, candidate{person: person, year: gedcomYear(date)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].year > candidates[j].year })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	people := make([]domain.Person, 0, len(candidates))
	for _, item := range candidates {
		people = append(people, item.person)
	}
	return people, nil
}

func gedcomYear(date string) int {
	for _, part := range strings.Fields(date) {
		if year, err := strconv.Atoi(part); err == nil && year >= 0 {
			return year
		}
	}
	return 0
}
