package db

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"webtrees-mcp/internal/domain"
	"webtrees-mcp/internal/genealogy"
)

type indexedDate struct {
	year  int
	month int
	day   int
}

func (r *Reader) SearchEvents(treeID, eventType, fromDate, toDate, place string, limit, offset int) ([]domain.EventSearchResult, error) {
	limit, offset = genealogy.NormalizePage(limit, offset)
	from, to, err := eventDateRange(fromDate, toDate)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT d_gid, d_type, d_year, d_month, d_day FROM %s_dates WHERE d_file = ? AND d_year >= ? AND d_year <= ? ORDER BY d_year DESC, d_month DESC, d_day DESC, d_gid ASC", r.prefix)
	rows, err := r.db.Query(query, treeID, from.year, to.year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]domain.EventSearchResult, 0, offset+limit)
	for rows.Next() {
		var id, typ string
		var date indexedDate
		if err := rows.Scan(&id, &typ, &date.year, &date.month, &date.day); err != nil {
			return nil, err
		}
		if !date.inRange(from, to) || (eventType != "" && !strings.EqualFold(typ, eventType)) {
			continue
		}
		person, err := r.GetPerson(treeID, id)
		if err != nil {
			return nil, err
		}
		for _, event := range person.Events {
			if !strings.EqualFold(event.Type, typ) || event.Date == nil || !eventDateMatches(event.Date.Raw, date) || (place != "" && !strings.Contains(strings.ToLower(event.Place), strings.ToLower(place))) {
				continue
			}
			results = append(results, domain.EventSearchResult{PersonID: id, Type: strings.ToLower(typ), Date: event.Date.Raw, Place: event.Place})
			break
		}
		if len(results) >= offset+limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if offset >= len(results) {
		return []domain.EventSearchResult{}, nil
	}
	return results[offset:], nil
}

func (d indexedDate) inRange(from, to indexedDate) bool {
	value := d.year*10000 + d.month*100 + d.day
	return value >= from.year*10000+from.month*100+from.day && value <= to.year*10000+to.month*100+to.day
}

func eventDateMatches(raw string, indexed indexedDate) bool {
	from, to, err := eventDateRange(raw, raw)
	if err != nil {
		return false
	}
	value := indexed.year*10000 + indexed.month*100 + indexed.day
	if value < from.year*10000+from.month*100+from.day || value > to.year*10000+to.month*100+to.day {
		return false
	}
	if from.year == to.year && from.month == to.month && from.day == to.day {
		return indexed.month == 0 || (indexed.month == from.month && (indexed.day == 0 || indexed.day == from.day))
	}
	return true
}

func eventDateRange(fromValue, toValue string) (indexedDate, indexedDate, error) {
	from, err := parseEventDate(fromValue, false)
	if err != nil {
		return indexedDate{}, indexedDate{}, err
	}
	to, err := parseEventDate(toValue, true)
	if err != nil {
		return indexedDate{}, indexedDate{}, err
	}
	if from.year > to.year || (from.year == to.year && (from.month > to.month || (from.month == to.month && from.day > to.day))) {
		return indexedDate{}, indexedDate{}, fmt.Errorf("from_date must not be after to_date")
	}
	return from, to, nil
}

func parseEventDate(value string, end bool) (indexedDate, error) {
	value = strings.TrimSpace(value)
	if len(value) > 10 && value[10] == 'T' {
		value = value[:10]
	}
	upper := strings.ToUpper(value)
	for _, prefix := range []string{"ABT ", "BEF ", "AFT ", "CAL ", "EST ", "INT "} {
		if strings.HasPrefix(upper, prefix) {
			value = strings.TrimSpace(value[len(prefix):])
			upper = strings.ToUpper(value)
			break
		}
	}
	for _, prefix := range []string{"BET ", "FROM "} {
		if strings.HasPrefix(upper, prefix) {
			parts := strings.SplitN(value[len(prefix):], " AND ", 2)
			if len(parts) == 2 {
				if end {
					value = strings.TrimSpace(parts[1])
				} else {
					value = strings.TrimSpace(parts[0])
				}
			}
			break
		}
	}
	if value == "" {
		if end {
			return indexedDate{year: 9999, month: 12, day: 31}, nil
		}
		return indexedDate{year: 0, month: 1, day: 1}, nil
	}
	value = strings.ReplaceAll(value, "/", "-")
	layouts := []string{"2006-01-02", "2006-01", "2006", "02 Jan 2006", "02 January 2006", "Jan 02 2006", "January 02 2006"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			date := indexedDate{year: parsed.Year(), month: int(parsed.Month()), day: parsed.Day()}
			if layout == "2006" {
				date.month, date.day = 1, 1
				if end {
					date.month, date.day = 12, 31
				}
			}
			if layout == "2006-01" && end {
				date.day = daysInMonth(date.year, date.month)
			}
			return date, nil
		}
	}
	return indexedDate{}, fmt.Errorf("unsupported date %q; use YYYY-MM-DD (preferred), YYYY, YYYY-MM, or a GEDCOM date such as 10 DEC 1815", value)
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

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

func (r *Reader) SearchPersons(criteria genealogy.PersonSearchCriteria) (genealogy.PersonSearchResults, error) {
	limit, offset := genealogy.NormalizePage(criteria.Limit, criteria.Offset)
	mode := criteria.MatchMode
	if mode == "" {
		mode = "prefix"
	}
	conditions := []string{"i.i_file = ?"}
	args := []any{criteria.TreeID}
	if criteria.Surname != "" {
		operator, value := nameMatchSQL(mode, criteria.Surname)
		conditions = append(conditions, "n.n_surname "+operator+" ?")
		args = append(args, value)
	}
	if criteria.GivenName != "" {
		operator, value := nameMatchSQL(mode, criteria.GivenName)
		conditions = append(conditions, "n.n_givn "+operator+" ?")
		args = append(args, value)
	}
	if criteria.Sex != "" {
		conditions = append(conditions, "i.i_sex = ?")
		args = append(args, criteria.Sex)
	}
	for _, filter := range []struct {
		column string
		value  *int
	}{
		{"i.i_birth_year >= ?", criteria.BirthYearMin},
		{"i.i_birth_year <= ?", criteria.BirthYearMax},
		{"i.i_death_year >= ?", criteria.DeathYearMin},
		{"i.i_death_year <= ?", criteria.DeathYearMax},
	} {
		column, value := filter.column, filter.value
		if value != nil {
			conditions = append(conditions, column)
			args = append(args, *value)
		}
	}
	where := strings.Join(conditions, " AND ")
	var totalCount int
	if mode != "fuzzy" {
		countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT i.i_id) FROM %s_individuals i INNER JOIN %s_name n ON n.n_file = i.i_file AND n.n_id = i.i_id WHERE %s", r.prefix, r.prefix, where)
		if err := r.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
			return genealogy.PersonSearchResults{}, err
		}
	}
	query := fmt.Sprintf("SELECT DISTINCT i.i_id, i.i_gedcom FROM %s_individuals i INNER JOIN %s_name n ON n.n_file = i.i_file AND n.n_id = i.i_id WHERE %s ORDER BY i.i_id LIMIT ? OFFSET ?", r.prefix, r.prefix, where)
	if mode == "fuzzy" {
		args = append(args, genealogy.MaxFuzzyCandidates, 0)
	} else {
		args = append(args, limit, offset)
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return genealogy.PersonSearchResults{}, err
	}
	defer rows.Close()
	people := make([]domain.PersonSearchResult, 0, limit)
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return genealogy.PersonSearchResults{}, err
		}
		person := parseIndividualGEDCOM(id, raw)
		result := domain.PersonSearchResult{Person: person, Match: domain.SearchMatch{DirectHit: true, Fields: []string{"surname"}}}
		if criteria.GivenName != "" {
			result.Match.Fields = append(result.Match.Fields, "given_name")
		}
		if mode == "fuzzy" {
			distance, matched := fuzzyPersonDistance(person, criteria.Surname, criteria.GivenName)
			if !matched {
				continue
			}
			result.Match.Fields = append(result.Match.Fields, fmt.Sprintf("fuzzy_distance:%d", distance))
			people = append(people, result)
			continue
		}
		people = append(people, result)
	}
	if err := rows.Err(); err != nil {
		return genealogy.PersonSearchResults{}, err
	}
	if mode == "fuzzy" {
		totalCount = len(people)
		sort.SliceStable(people, func(i, j int) bool {
			return fuzzyDistance(people[i].Match.Fields) < fuzzyDistance(people[j].Match.Fields)
		})
		if offset >= len(people) {
			return genealogy.PersonSearchResults{People: []domain.PersonSearchResult{}, TotalCount: totalCount}, nil
		}
		people = people[offset:]
		if len(people) > limit {
			people = people[:limit]
		}
	}
	return genealogy.PersonSearchResults{People: people, TotalCount: totalCount}, nil
}

func nameMatchSQL(mode, value string) (string, string) {
	switch mode {
	case "exact":
		return "=", value
	case "fuzzy":
		return "LIKE", value[:1] + "%"
	default:
		return "LIKE", value + "%"
	}
}

func fuzzyPersonDistance(person domain.Person, surname, givenName string) (int, bool) {
	best := genealogy.MaxFuzzyCandidates
	for _, name := range person.Names {
		distance := levenshtein(strings.ToLower(name.Surname), strings.ToLower(surname))
		if givenName != "" {
			distance += levenshtein(strings.ToLower(name.Given), strings.ToLower(givenName))
		}
		if distance < best {
			best = distance
		}
	}
	if len(person.Names) == 0 {
		best = levenshtein(strings.ToLower(person.Name.Surname), strings.ToLower(surname))
		if givenName != "" {
			best += levenshtein(strings.ToLower(person.Name.Given), strings.ToLower(givenName))
		}
	}
	return best, best <= 2
}

func levenshtein(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, leftRune := range a {
		current := []int{i + 1}
		for j, rightRune := range b {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current = append(current, min(current[j]+1, previous[j+1]+1, previous[j]+cost))
		}
		previous = current
	}
	return previous[len(b)]
}

func fuzzyDistance(fields []string) int {
	for _, field := range fields {
		if strings.HasPrefix(field, "fuzzy_distance:") {
			var distance int
			_, _ = fmt.Sscanf(field, "fuzzy_distance:%d", &distance)
			return distance
		}
	}
	return genealogy.MaxFuzzyCandidates
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
