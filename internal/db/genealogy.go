package db

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"webtrees-mcp/internal/domain"
	"webtrees-mcp/internal/genealogy"
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

func (r *Reader) SearchPersons(treeID, surname string, includeIndirect bool, limit, offset int) ([]domain.PersonSearchResult, error) {
	limit, offset = genealogy.NormalizePage(limit, offset)
	pageEnd := offset + limit
	query := fmt.Sprintf("SELECT i_id, i_gedcom FROM %s_individuals WHERE i_file = ? AND i_gedcom LIKE ? ORDER BY i_id", r.prefix)
	rows, err := r.db.Query(query, treeID, "%"+surname+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := make([]domain.PersonSearchResult, 0, pageEnd)
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		person := parseIndividualGEDCOM(id, raw)
		result := domain.PersonSearchResult{Person: person, Match: searchMatch(person, surname)}
		if !includeIndirect && !result.Match.DirectHit {
			continue
		}
		people = append(people, result)
		sort.Slice(people, func(i, j int) bool {
			if people[i].Match.DirectHit != people[j].Match.DirectHit {
				return people[i].Match.DirectHit
			}
			return people[i].Person.ID < people[j].Person.ID
		})
		if len(people) > pageEnd {
			people = people[:pageEnd]
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if offset >= len(people) {
		return []domain.PersonSearchResult{}, nil
	}
	return people[offset:], nil
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

func (r *Reader) ListTrees(limit, offset int) ([]domain.Tree, error) {
	query := fmt.Sprintf("SELECT gedcom_id, gedcom_name, title FROM %s_gedcom ORDER BY gedcom_id LIMIT ? OFFSET ?", r.prefix)
	limit, offset = genealogy.NormalizePage(limit, offset)
	rows, err := r.db.Query(query, limit, offset)
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

func (r *Reader) ListRecentlyBorn(treeID string, limit, offset int) ([]domain.Person, error) {
	return r.listByEvent(treeID, limit, offset, true)
}

func (r *Reader) ListRecentlyDeceased(treeID string, limit, offset int) ([]domain.Person, error) {
	return r.listByEvent(treeID, limit, offset, false)
}

func (r *Reader) listByEvent(treeID string, limit, offset int, born bool) ([]domain.Person, error) {
	limit, offset = genealogy.NormalizePage(limit, offset)
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
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].year != candidates[j].year {
			return candidates[i].year > candidates[j].year
		}
		return candidates[i].person.ID < candidates[j].person.ID
	})
	if offset >= len(candidates) {
		return []domain.Person{}, nil
	}
	candidates = candidates[offset:]
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
