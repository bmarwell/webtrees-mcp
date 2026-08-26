package mcp

import (
	"context"
	"fmt"
	"strings"

	"webtrees-mcp/internal/domain"
	"webtrees-mcp/internal/genealogy"

	framework "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(s *server.MCPServer, reader genealogy.Repository) {
	tool := framework.NewTool("get_person",
		framework.WithDescription("Use when you have an exact person_id and need that person's dates, events, alternate names, family links, notes, or sources. Do not use for name searches; call search_persons first and then chain the returned ID here."),
		framework.WithOutputSchema[personResultDTO](),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("person_id", framework.Required(), framework.Description("Individual xref, for example I1")),
	)
	s.AddTool(tool, getPersonHandler(reader))
	s.AddTool(framework.NewTool("search_persons",
		framework.WithDescription("Use when you need to find people by indexed name, sex, or birth/death year bounds. Required: non-blank tree_id and surname. Optional: given_name, sex, birth_year_min, birth_year_max, death_year_min, death_year_max, limit (default 10, range 1-100), and offset (default 0, range 0-10000). Do not use this as a raw GEDCOM search; filters are applied to indexed fields and results are research leads until verified. Chain selected person IDs into get_person for full evidence."),
		framework.WithOutputSchema[peopleResultDTO](),
		framework.WithString("tree_id", framework.Required(), framework.MinLength(1), framework.Description("Required, non-blank webtrees tree ID")),
		framework.WithString("surname", framework.Required(), framework.MinLength(1), framework.Description("Required, non-blank surname or name fragment; surrounding whitespace is ignored")),
		framework.WithString("given_name", framework.MinLength(1), framework.Description("Optional given-name fragment, matched against the indexed given-name field")),
		framework.WithString("sex", framework.MinLength(1), framework.Description("Optional indexed sex value, for example F or M")),
		framework.WithBoolean("include_indirect", framework.DefaultBool(false), framework.Description("Deprecated compatibility option; indexed searches do not scan indirect GEDCOM text")),
		framework.WithInteger("birth_year_min", framework.Description("Optional inclusive minimum indexed birth year")),
		framework.WithInteger("birth_year_max", framework.Description("Optional inclusive maximum indexed birth year")),
		framework.WithInteger("death_year_min", framework.Description("Optional inclusive minimum indexed death year")),
		framework.WithInteger("death_year_max", framework.Description("Optional inclusive maximum indexed death year")),
		framework.WithInteger("limit", framework.DefaultNumber(genealogy.DefaultPageSize), framework.Min(1), framework.Max(genealogy.MaxPageSize), framework.Description("Maximum people to return (1-100)")),
		framework.WithInteger("offset", framework.DefaultNumber(0), framework.Min(0), framework.Max(genealogy.MaxPageOffset), framework.Description("Number of matching people to skip (0-10000)")),
	), searchPersonsHandler(reader))
	s.AddTool(framework.NewTool("get_family",
		framework.WithDescription("Use when you have an exact family_id and need its parent/child links, family events, notes, or sources. Chain the returned person IDs into get_person; do not infer relationships from surnames."),
		framework.WithOutputSchema[familyOutputDTO](),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("family_id", framework.Required(), framework.Description("Family xref, for example F1")),
	), getFamilyHandler(reader))
	s.AddTool(framework.NewTool("relationship_path",
		framework.WithDescription("Use when you need an evidence-backed relationship path between two known individuals in one tree. Do not infer a relationship from surnames or incomplete data; this follows explicit family links and may return no path within the bounded search. Chain each returned person_id or family_id into get_person or get_family for detail."),
		framework.WithOutputSchema[relationshipPathResultDTO](),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("from_person_id", framework.Required(), framework.Description("Starting individual xref, for example I1")),
		framework.WithString("to_person_id", framework.Required(), framework.Description("Target individual xref, for example I2")),
	), relationshipPathHandler(reader))
	s.AddTool(framework.NewTool("search_events",
		framework.WithDescription("Use when searching indexed individual events by type, date range, or place. Do not treat an indexed year as more precise than the source date, and use the preferred YYYY-MM-DD format when possible. Results are bounded and paged; chain returned person_id values into get_person for full evidence."),
		framework.WithOutputSchema[eventSearchResultsDTO](),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("event_type", framework.Description("GEDCOM event type, for example BIRT, DEAT, or MARR")),
		framework.WithString("from_date", framework.Description("Inclusive lower date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("to_date", framework.Description("Inclusive upper date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("place", framework.Description("Case-insensitive place substring")),
		framework.WithInteger("limit", framework.Description("Maximum events; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of matching events to skip; defaults to 0")),
	), searchEventsHandler(reader))
	s.AddTool(framework.NewTool("list_tree_ids",
		framework.WithDescription("Use when the tree_id is unknown or when choosing among multiple trees. Do not use after the tree is selected. Results are ordered by tree ID and paged with limit and offset. Chain the selected tree_id into every subsequent genealogy query."),
		framework.WithOutputSchema[treesResultDTO](),
		framework.WithInteger("limit", framework.Description("Maximum number of trees; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of trees to skip; defaults to 0"))), listTreesHandler(reader))
	for _, spec := range []struct {
		name string
		desc string
		fn   func(genealogy.Repository, string, int, int) ([]domain.Person, error)
	}{
		{"list_recently_born", "Use when looking for recorded births in a tree. Do not treat the ranking as proof of the historically latest event. Results are ordered by parsed birth year and person ID, and paged with limit and offset; chain returned person IDs into get_person for event precision and evidence.", func(r genealogy.Repository, treeID string, limit, offset int) ([]domain.Person, error) {
			return r.ListRecentlyBorn(treeID, limit, offset)
		}},
		{"list_recently_deceased", "Use when looking for recorded deaths in a tree. Do not treat the ranking as proof of the historically latest event. Results are ordered by parsed death year and person ID, and paged with limit and offset; chain returned person IDs into get_person for event precision and evidence.", func(r genealogy.Repository, treeID string, limit, offset int) ([]domain.Person, error) {
			return r.ListRecentlyDeceased(treeID, limit, offset)
		}},
	} {
		s.AddTool(framework.NewTool(spec.name, framework.WithDescription(spec.desc),
			framework.WithOutputSchema[peopleResultDTO](),
			framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
			framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10 and is capped at 100")),
			framework.WithInteger("offset", framework.Description("Number of people to skip; defaults to 0")),
		), listPeopleHandler(reader, spec.fn))
	}
}

func searchEventsHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID    string `json:"tree_id"`
			EventType string `json:"event_type"`
			FromDate  string `json:"from_date"`
			ToDate    string `json:"to_date"`
			Place     string `json:"place"`
			Limit     int    `json:"limit"`
			Offset    int    `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" {
			return framework.NewToolResultError("tree_id is required"), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		events, err := reader.SearchEvents(args.TreeID, args.EventType, args.FromDate, args.ToDate, args.Place, args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(eventSearchResults(events), eventSearchSummary(events))
	}
}

func relationshipPathHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID       string `json:"tree_id"`
			FromPersonID string `json:"from_person_id"`
			ToPersonID   string `json:"to_person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" || args.FromPersonID == "" || args.ToPersonID == "" {
			return framework.NewToolResultError("tree_id, from_person_id, and to_person_id are required"), nil
		}
		path, found, err := genealogy.FindRelationshipPath(reader, args.TreeID, args.FromPersonID, args.ToPersonID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(relationshipPathResult(args.FromPersonID, args.ToPersonID, found, path), relationshipPathSummary(args.FromPersonID, args.ToPersonID, found, path))
	}
}

func relationshipPathSummary(fromID, toID string, found bool, path []domain.RelationshipPathStep) string {
	if !found {
		return fmt.Sprintf("Relationship path: none found\nFrom: %s\nTo: %s", fromID, toID)
	}
	lines := []string{fmt.Sprintf("Relationship path: %s -> %s", fromID, toID), fmt.Sprintf("Hops: %d", len(path))}
	for _, step := range path {
		lines = append(lines, fmt.Sprintf("- %s -> %s via %s (%s)", step.FromPersonID, step.ToPersonID, step.FamilyID, step.Relationship))
	}
	return strings.Join(lines, "\n")
}

func eventSearchSummary(events []domain.EventSearchResult) string {
	if len(events) == 0 {
		return "Events: none found."
	}
	lines := []string{fmt.Sprintf("Events: %d", len(events))}
	for _, event := range events {
		line := fmt.Sprintf("- Person: %s; Type: %s; Date: %s", event.PersonID, strings.ToUpper(event.Type), event.Date)
		if event.Place != "" {
			line += "; Place: " + event.Place
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func searchPersonsHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID          string `json:"tree_id"`
			Surname         string `json:"surname"`
			GivenName       string `json:"given_name"`
			Sex             string `json:"sex"`
			IncludeIndirect bool   `json:"include_indirect"`
			BirthYearMin    *int   `json:"birth_year_min"`
			BirthYearMax    *int   `json:"birth_year_max"`
			DeathYearMin    *int   `json:"death_year_min"`
			DeathYearMax    *int   `json:"death_year_max"`
			Limit           int    `json:"limit"`
			Offset          int    `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		args.TreeID = strings.TrimSpace(args.TreeID)
		args.Surname = strings.TrimSpace(args.Surname)
		if args.TreeID == "" {
			return framework.NewToolResultError("tree_id must not be blank"), nil
		}
		if args.Surname == "" {
			return framework.NewToolResultError("surname must not be blank"), nil
		}
		args.GivenName = strings.TrimSpace(args.GivenName)
		args.Sex = strings.TrimSpace(args.Sex)
		if args.Limit < 0 || args.Limit > genealogy.MaxPageSize {
			return framework.NewToolResultError(fmt.Sprintf("limit must be between 1 and %d (or omitted)", genealogy.MaxPageSize)), nil
		}
		if args.Offset < 0 || args.Offset > genealogy.MaxPageOffset {
			return framework.NewToolResultError(fmt.Sprintf("offset must be between 0 and %d", genealogy.MaxPageOffset)), nil
		}
		if args.BirthYearMin != nil && args.BirthYearMax != nil && *args.BirthYearMin > *args.BirthYearMax {
			return framework.NewToolResultError("birth_year_min must not be greater than birth_year_max"), nil
		}
		if args.DeathYearMin != nil && args.DeathYearMax != nil && *args.DeathYearMin > *args.DeathYearMax {
			return framework.NewToolResultError("death_year_min must not be greater than death_year_max"), nil
		}
		results, err := reader.SearchPersons(genealogy.PersonSearchCriteria{
			TreeID: args.TreeID, Surname: args.Surname, GivenName: args.GivenName, Sex: args.Sex,
			BirthYearMin: args.BirthYearMin, BirthYearMax: args.BirthYearMax,
			DeathYearMin: args.DeathYearMin, DeathYearMax: args.DeathYearMax,
			IncludeIndirect: args.IncludeIndirect, Limit: args.Limit, Offset: args.Offset,
		})
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		people := make([]domain.Person, 0, len(results))
		for _, result := range results {
			people = append(people, result.Person)
		}
		return structuredResult(peopleResultFromOutputs(searchPersonOutputs(results)), peopleSummary(people, fmt.Sprintf("Found %d people matching search %q.", len(people), args.Surname)))
	}
}

func getFamilyHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID   string `json:"tree_id"`
			FamilyID string `json:"family_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" || args.FamilyID == "" {
			return framework.NewToolResultError("tree_id and family_id are required"), nil
		}
		family, err := reader.GetFamily(args.TreeID, args.FamilyID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(familyOutput(*family), familySummary(*family))
	}
}

func listTreesHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		trees, err := reader.ListTrees(args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(treesResult(trees), treesSummary(trees))
	}
}

func listPeopleHandler(reader genealogy.Repository, list func(genealogy.Repository, string, int, int) ([]domain.Person, error)) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID string `json:"tree_id"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" {
			return framework.NewToolResultError("tree_id is required"), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		people, err := list(reader, args.TreeID, args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(peopleResult(people), peopleSummary(people, fmt.Sprintf("Found %d people.", len(people))))
	}
}

func structuredResult(value any, summary string) (*framework.CallToolResult, error) {
	return framework.NewToolResultStructured(value, summary), nil
}

func getPersonHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(ctx context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID   string `json:"tree_id"`
			PersonID string `json:"person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" || args.PersonID == "" {
			return framework.NewToolResultError("tree_id and person_id are required"), nil
		}
		person, err := reader.GetPerson(args.TreeID, args.PersonID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(personResult(*person), personSummary(*person))
	}
}

func personSummary(person domain.Person) string {
	name := displayName(person)
	if name == "" {
		name = "Unknown person"
	}
	lines := []string{
		"Person ID: " + person.ID,
		"Name: " + name,
		"Sex: " + valueOrNotRecorded(person.Sex),
		"Birth date: " + valueOrNotRecorded(person.BirthDate),
		"Death date: " + valueOrNotRecorded(person.DeathDate),
		"Occupation: " + valueOrNotRecorded(person.Occupation),
	}
	lines = append(lines, personEventSummaryLines(person)...)
	if len(person.Names) > 1 {
		lines = append(lines, "Alternate names:")
		for _, alternate := range person.Names[1:] {
			lines = append(lines, "- "+displayNameValue(alternate))
		}
	} else {
		lines = append(lines, "Alternate names: none")
	}
	if len(person.Relatives) > 0 {
		lines = append(lines, "Relatives:")
		for _, relative := range person.Relatives {
			description := relative.PersonID
			if relative.Relationship != "" {
				description += " (" + relative.Relationship + ")"
			}
			lines = append(lines, "- "+description)
		}
	} else {
		lines = append(lines, "Relatives: none")
	}
	if len(person.FamilyLinks) > 0 {
		lines = append(lines, "Family links:")
		for _, link := range person.FamilyLinks {
			lines = append(lines, "- "+link.FamilyID+" ("+link.Role+")")
		}
	} else {
		lines = append(lines, "Family links: none")
	}
	if len(person.Notes) > 0 {
		lines = append(lines, "Notes:")
		for _, note := range person.Notes {
			lines = append(lines, "- "+note)
		}
	} else {
		lines = append(lines, "Notes: none")
	}
	if len(person.Sources) > 0 {
		lines = append(lines, "Sources:")
		for _, source := range person.Sources {
			line := source.ID
			if source.Title != "" {
				line += " (" + source.Title + ")"
			}
			lines = append(lines, "- "+line)
		}
	} else {
		lines = append(lines, "Sources: none")
	}
	return strings.Join(lines, "\n")
}

func valueOrNotRecorded(value string) string {
	if value == "" {
		return "not recorded"
	}
	return value
}

func personEventSummaryLines(person domain.Person) []string {
	lines := []string{"Events:"}
	hasBirth, hasDeath, hasOccupation := false, false, false
	for _, event := range person.Events {
		tag := strings.ToUpper(event.Type)
		if tag == "BIRT" {
			hasBirth = true
		}
		if tag == "DEAT" {
			hasDeath = true
		}
		if tag == "OCCU" {
			hasOccupation = true
		}
		lines = appendEventSummaryLines(lines, event)
	}
	if person.BirthDate != "" && !hasBirth {
		date := person.BirthDate
		lines = appendEventSummaryLines(lines, domain.Event{Type: "BIRT", Date: &domain.Date{Raw: date}})
	}
	if person.DeathDate != "" && !hasDeath {
		lines = appendEventSummaryLines(lines, domain.Event{Type: "DEAT", Date: &domain.Date{Raw: person.DeathDate}})
	}
	if person.Occupation != "" && !hasOccupation {
		lines = appendEventSummaryLines(lines, domain.Event{Type: "OCCU", Value: person.Occupation})
	}
	if len(lines) == 1 {
		return []string{"Events: none"}
	}
	return lines
}

func appendEventSummaryLines(lines []string, event domain.Event) []string {
	date, value := "not recorded", "not recorded"
	if event.Date != nil {
		date = event.Date.Raw
	}
	if event.Value != "" {
		value = event.Value
	}
	lines = append(lines, "- "+strings.ToUpper(event.Type)+":", "  Date: "+date, "  Place: "+valueOrNotRecorded(event.Place), "  Value: "+value)
	if len(event.Notes) == 0 {
		lines = append(lines, "  Notes: none")
	} else {
		lines = append(lines, "  Notes:")
		for _, note := range event.Notes {
			lines = append(lines, "  - "+note)
		}
	}
	if len(event.Sources) == 0 {
		lines = append(lines, "  Sources: none")
	} else {
		lines = append(lines, "  Sources:")
		for _, source := range event.Sources {
			line := source.ID
			if source.Title != "" {
				line += " (" + source.Title + ")"
			}
			lines = append(lines, "  - "+line)
		}
	}
	return lines
}

func displayNameValue(name domain.Name) string {
	value := strings.TrimSpace(strings.Join([]string{name.Given, name.Surname}, " "))
	if name.Type != "" {
		value += " (" + name.Type + ")"
	}
	return value
}

func joinPhrases(phrases []string) string {
	switch len(phrases) {
	case 0:
		return ""
	case 1:
		return phrases[0]
	case 2:
		return phrases[0] + " and " + phrases[1]
	default:
		return strings.Join(phrases[:len(phrases)-1], ", ") + ", and " + phrases[len(phrases)-1]
	}
}

func displayName(person domain.Person) string {
	return strings.TrimSpace(strings.Join([]string{person.Name.Given, person.Name.Surname}, " "))
}

func peopleSummary(people []domain.Person, prefix string) string {
	if len(people) == 0 {
		return prefix
	}
	summaries := make([]string, 0, len(people))
	for _, person := range people {
		summaries = append(summaries, personSummary(person))
	}
	return prefix + "\n\n" + strings.Join(summaries, "\n\n")
}

func familySummary(family domain.Family) string {
	lines := []string{"Family ID: " + family.ID}
	if len(family.Parents) > 0 {
		lines = append(lines, "Parents:")
		for _, parent := range family.Parents {
			lines = append(lines, "- "+parent.PersonID)
		}
	} else {
		lines = append(lines, "Parents: none")
	}
	if len(family.Children) > 0 {
		lines = append(lines, "Children:")
		for _, child := range family.Children {
			lines = append(lines, "- "+child.PersonID)
		}
	} else {
		lines = append(lines, "Children: none")
	}
	if len(family.Events) > 0 {
		lines = append(lines, "Events:")
		for _, event := range family.Events {
			lines = appendEventSummaryLines(lines, event)
		}
	} else {
		lines = append(lines, "Events: none")
	}
	if len(family.Notes) > 0 {
		lines = append(lines, "Notes:")
		for _, note := range family.Notes {
			lines = append(lines, "- "+note)
		}
	} else {
		lines = append(lines, "Notes: none")
	}
	if len(family.Sources) > 0 {
		lines = append(lines, "Sources:")
		for _, source := range family.Sources {
			line := source.ID
			if source.Title != "" {
				line += " (" + source.Title + ")"
			}
			lines = append(lines, "- "+line)
		}
	} else {
		lines = append(lines, "Sources: none")
	}
	return strings.Join(lines, "\n")
}

func relativeIDs(relatives []domain.Relative) []string {
	ids := make([]string, 0, len(relatives))
	for _, relative := range relatives {
		ids = append(ids, relative.PersonID)
	}
	return ids
}

func treesSummary(trees []domain.Tree) string {
	if len(trees) == 0 {
		return "Trees: none found."
	}
	lines := []string{fmt.Sprintf("Trees: %d", len(trees))}
	for _, tree := range trees {
		description := "- " + tree.ID
		if tree.Name != "" {
			description += "; name: " + tree.Name
		}
		if tree.Title != "" {
			description += "; title: " + tree.Title
		}
		lines = append(lines, description)
	}
	return strings.Join(lines, "\n")
}
