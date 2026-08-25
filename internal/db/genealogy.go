package db

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"webtrees-mcp/internal/model"
)

// GetPerson reads one individual. The table name is validated when Reader is
// constructed; values remain regular SQL parameters.
func (r *Reader) GetPerson(treeID, xref string) (*model.PersonDTO, error) {
	query := fmt.Sprintf("SELECT i_gedcom FROM %s_individuals WHERE i_file = ? AND i_id = ?", r.prefix)
	var raw string
	if err := r.db.QueryRow(query, treeID, xref).Scan(&raw); err != nil {
		return nil, err
	}
	person := model.ParseIndividualGEDCOM(xref, raw)
	return &person, nil
}

func (r *Reader) SearchPersons(treeID, surname string) ([]model.PersonDTO, error) {
	query := fmt.Sprintf("SELECT i_id, i_gedcom FROM %s_individuals WHERE i_file = ? AND i_gedcom LIKE ? ORDER BY i_id", r.prefix)
	rows, err := r.db.Query(query, treeID, "%"+surname+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var people []model.PersonDTO
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		people = append(people, model.ParseIndividualGEDCOM(id, raw))
	}
	return people, rows.Err()
}

func (r *Reader) GetFamily(treeID, xref string) (*model.FamilyDTO, error) {
	query := fmt.Sprintf("SELECT f_gedcom FROM %s_families WHERE f_file = ? AND f_id = ?", r.prefix)
	var raw string
	if err := r.db.QueryRow(query, treeID, xref).Scan(&raw); err != nil {
		return nil, err
	}
	family := model.ParseFamilyGEDCOM(xref, raw)
	return &family, nil
}

func (r *Reader) ListTrees() ([]model.TreeDTO, error) {
	query := fmt.Sprintf("SELECT gedcom_id, gedcom_name, title FROM %s_gedcom ORDER BY gedcom_id", r.prefix)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trees []model.TreeDTO
	for rows.Next() {
		var tree model.TreeDTO
		if err := rows.Scan(&tree.ID, &tree.Name, &tree.Title); err != nil {
			return nil, err
		}
		trees = append(trees, tree)
	}
	return trees, rows.Err()
}

func (r *Reader) ListRecentlyBorn(treeID string, limit int) ([]model.PersonDTO, error) {
	return r.listByEvent(treeID, limit, true)
}

func (r *Reader) ListRecentlyDeceased(treeID string, limit int) ([]model.PersonDTO, error) {
	return r.listByEvent(treeID, limit, false)
}

func (r *Reader) listByEvent(treeID string, limit int, born bool) ([]model.PersonDTO, error) {
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
		person model.PersonDTO
		year   int
	}
	var candidates []candidate
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		person := model.ParseIndividualGEDCOM(id, raw)
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
	people := make([]model.PersonDTO, 0, len(candidates))
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
