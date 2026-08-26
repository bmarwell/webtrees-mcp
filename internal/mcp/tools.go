package mcp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"webtrees-mcp/internal/domain"
	"webtrees-mcp/internal/genealogy"

	framework "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(s *server.MCPServer, reader genealogy.Repository) {
	tool := framework.NewTool("get_person_by_exact_id",
		framework.WithDescription("Use when you already have an exact GEDCOM person_id such as I123 and need verified details. Do not pass a person's name here and do not use for name searches; call search_person_by_name first. Chain the returned person_id here. Read content as the complete factual result; do not infer omitted facts."),
		framework.WithOutputSchema[personResultDTO](),
		framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42; do not pass a tree name or the literal default")),
		framework.WithString("person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name; search by name first.")),
	)
	s.AddTool(tool, getPersonByExactIDHandler(reader))
	s.AddTool(framework.NewTool("search_person_by_name",
		framework.WithDescription("Use when you have a person's name and need a research lead. Required: tree_id and surname; optional: given_name, sex, match_mode (exact, prefix, or fuzzy; default prefix), birth_year_min/max, death_year_min/max, limit 1-100 (default 10), offset 0-10000 (default 0). Do not pass a name to get_person_by_exact_id or treat results as verified facts; fuzzy mode is bounded and may miss candidates. Chain a returned person_id into get_person_by_exact_id. Read content as the factual lead record."),
		framework.WithOutputSchema[searchPeopleResultDTO](),
		framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42; do not pass a tree name or the literal default")),
		framework.WithString("surname", framework.Required(), framework.MinLength(1), framework.Description("Required, non-blank surname or name fragment; surrounding whitespace is ignored")),
		framework.WithString("given_name", framework.MinLength(1), framework.Description("Optional given-name fragment, matched against the indexed given-name field")),
		framework.WithString("sex", framework.MinLength(1), framework.Description("Optional indexed sex value, for example F or M")),
		framework.WithString("match_mode", framework.Enum("exact", "prefix", "fuzzy"), framework.DefaultString("prefix"), framework.Description("Name matching strategy; exact equality, indexed prefix, or bounded fuzzy matching")),
		framework.WithBoolean("include_indirect", framework.DefaultBool(false), framework.Description("Deprecated compatibility option; indexed searches do not scan indirect GEDCOM text")),
		framework.WithInteger("birth_year_min", framework.Description("Optional inclusive minimum indexed birth year")),
		framework.WithInteger("birth_year_max", framework.Description("Optional inclusive maximum indexed birth year")),
		framework.WithInteger("death_year_min", framework.Description("Optional inclusive minimum indexed death year")),
		framework.WithInteger("death_year_max", framework.Description("Optional inclusive maximum indexed death year")),
		framework.WithInteger("limit", framework.DefaultNumber(genealogy.DefaultPageSize), framework.Min(1), framework.Max(genealogy.MaxPageSize), framework.Description("Maximum people to return (1-100)")),
		framework.WithInteger("offset", framework.DefaultNumber(0), framework.Min(0), framework.Max(genealogy.MaxPageOffset), framework.Description("Number of matching people to skip (0-10000)")),
	), searchPersonByNameHandler(reader))
	s.AddTool(framework.NewTool("get_family",
		framework.WithDescription("Use when you have an exact family_id and need its links and family evidence. Do not infer relationships from surnames or missing links. Chain returned person_id values into get_person_by_exact_id."),
		framework.WithOutputSchema[familyOutputDTO](),
		framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42")),
		framework.WithString("family_id", framework.Required(), framework.Description("Family xref, for example F1")),
	), getFamilyHandler(reader))
	s.AddTool(framework.NewTool("relationship_path",
		framework.WithDescription("Use when you need an evidence-backed path between two known individuals. Do not infer a relationship from surnames or incomplete data; this bounded search may find no path. Chain returned person_id or family_id values into get_person_by_exact_id or get_family."),
		framework.WithOutputSchema[relationshipPathResultDTO](),
		framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42")),
		framework.WithString("from_person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name.")),
		framework.WithString("to_person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I456. Do not pass a person's name.")),
	), relationshipPathHandler(reader))
	lineageTools := []struct {
		name        string
		direction   string
		description string
	}{
		{"get_ancestors", "ancestors", "Use when you need the direct ancestor line for a known person. Do not infer ancestors from surnames or incomplete family links; traversal is bounded by depth and limit. Chain returned person_id and via_family_id values into get_person_by_exact_id and get_family."},
		{"get_descendants", "descendants", "Use when you need the direct descendant line for a known person. Do not infer descendants from surnames or incomplete family links; traversal is bounded by depth and limit. Chain returned person_id and via_family_id values into get_person_by_exact_id and get_family."},
	}
	for _, spec := range lineageTools {
		s.AddTool(framework.NewTool(spec.name,
			framework.WithDescription(spec.description), framework.WithOutputSchema[lineageResultDTO](),
			framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42")),
			framework.WithString("person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name; search by name first.")),
			framework.WithInteger("max_depth", framework.DefaultNumber(genealogy.DefaultLineageDepth), framework.Min(1), framework.Max(genealogy.MaxLineageDepth), framework.Description("Maximum number of generations to traverse")),
			framework.WithInteger("limit", framework.DefaultNumber(genealogy.DefaultLineageLimit), framework.Min(1), framework.Max(genealogy.MaxLineageLimit), framework.Description("Maximum people to return")),
		), lineageHandler(reader, spec.direction))
	}
	s.AddTool(framework.NewTool("search_events",
		framework.WithDescription("Use when searching indexed individual events by type, date range, or place. Do not treat an indexed year as more precise than the source date; results are leads, not proof. Chain returned person_id values into get_person_by_exact_id for full evidence."),
		framework.WithOutputSchema[eventSearchResultsDTO](),
		framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42")),
		framework.WithString("event_type", framework.Description("GEDCOM event type, for example BIRT, DEAT, or MARR")),
		framework.WithString("from_date", framework.Description("Inclusive lower date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("to_date", framework.Description("Inclusive upper date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("place", framework.Description("Case-insensitive place substring")),
		framework.WithInteger("limit", framework.Description("Maximum events; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of matching events to skip; defaults to 0")),
	), searchEventsHandler(reader))
	s.AddTool(framework.NewTool("list_tree_ids",
		framework.WithDescription("Use when tree_id is unknown or you must choose a tree. Do not use after selecting a tree. Results are ordered and paged. Chain the selected tree_id into every subsequent genealogy query."),
		framework.WithOutputSchema[treesResultDTO](),
		framework.WithInteger("limit", framework.Description("Maximum number of trees; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of trees to skip; defaults to 0"))), listTreesHandler(reader))
	for _, spec := range []struct {
		name string
		desc string
		fn   func(genealogy.Repository, string, int, int) ([]domain.Person, error)
	}{
		{"list_recently_born", "Use when looking for recorded births in a tree. Do not treat ranking as proof of the historically latest event; results are research leads ordered by parsed birth year and person ID. Chain returned person IDs into get_person_by_exact_id for evidence.", func(r genealogy.Repository, treeID string, limit, offset int) ([]domain.Person, error) {
			return r.ListRecentlyBorn(treeID, limit, offset)
		}},
		{"list_recently_deceased", "Use when looking for recorded deaths in a tree. Do not treat ranking as proof of the historically latest event; results are research leads ordered by parsed death year and person ID. Chain returned person IDs into get_person_by_exact_id for evidence.", func(r genealogy.Repository, treeID string, limit, offset int) ([]domain.Person, error) {
			return r.ListRecentlyDeceased(treeID, limit, offset)
		}},
	} {
		s.AddTool(framework.NewTool(spec.name, framework.WithDescription(spec.desc),
			framework.WithOutputSchema[peopleResultDTO](),
			framework.WithInteger("tree_id", framework.Required(), framework.Min(1), framework.Description("Required numeric webtrees tree ID, for example 42")),
			framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10 and is capped at 100")),
			framework.WithInteger("offset", framework.Description("Number of people to skip; defaults to 0")),
		), listPeopleHandler(reader, spec.fn))
	}
}

func searchEventsHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID    int    `json:"tree_id"`
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
		if args.TreeID < 1 {
			return framework.NewToolResultError("tree_id must be a positive numeric ID"), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		events, err := reader.SearchEvents(strconv.Itoa(args.TreeID), args.EventType, args.FromDate, args.ToDate, args.Place, args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(eventSearchResults(events), eventSearchSummary(events))
	}
}

func relationshipPathHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID       int    `json:"tree_id"`
			FromPersonID string `json:"from_person_id"`
			ToPersonID   string `json:"to_person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID < 1 || !validPersonID(args.FromPersonID) || !validPersonID(args.ToPersonID) {
			return framework.NewToolResultError("tree_id must be a positive numeric ID; from_person_id and to_person_id are required"), nil
		}
		path, found, err := genealogy.FindRelationshipPath(reader, strconv.Itoa(args.TreeID), args.FromPersonID, args.ToPersonID)
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

func lineageHandler(reader genealogy.Repository, direction string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID   int    `json:"tree_id"`
			PersonID string `json:"person_id"`
			MaxDepth int    `json:"max_depth"`
			Limit    int    `json:"limit"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID < 1 || !validPersonID(args.PersonID) {
			return framework.NewToolResultError("tree_id must be a positive numeric ID and person_id is required"), nil
		}
		if args.MaxDepth < 0 || args.MaxDepth > genealogy.MaxLineageDepth || args.Limit < 0 || args.Limit > genealogy.MaxLineageLimit {
			return framework.NewToolResultError(fmt.Sprintf("max_depth must be 1-%d and limit must be 1-%d (or omitted)", genealogy.MaxLineageDepth, genealogy.MaxLineageLimit)), nil
		}
		rootID := strings.TrimSpace(args.PersonID)
		var result domain.LineageResult
		var err error
		if direction == "ancestors" {
			result, err = genealogy.FindAncestors(reader, strconv.Itoa(args.TreeID), rootID, args.MaxDepth, args.Limit)
		} else {
			result, err = genealogy.FindDescendants(reader, strconv.Itoa(args.TreeID), rootID, args.MaxDepth, args.Limit)
		}
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(lineageResult(result), lineageSummary(result))
	}
}

func lineageSummary(result domain.LineageResult) string {
	lines := []string{"Lineage: " + result.Direction, "Root person ID: " + result.RootPersonID, fmt.Sprintf("Nodes: %d", len(result.Nodes)), "Truncated: " + fmt.Sprint(result.Truncated)}
	if len(result.Nodes) == 0 {
		lines = append(lines, "Nodes: none")
		return strings.Join(lines, "\n")
	}
	for _, node := range result.Nodes {
		lines = append(lines, fmt.Sprintf("Node: person_id=%s; depth=%d; via_family_id=%s; relationship=%s", node.Person.ID, node.Depth, node.ViaFamilyID, node.Relationship), personSummary(node.Person))
	}
	return strings.Join(lines, "\n\n")
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

func searchPersonByNameHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID          int    `json:"tree_id"`
			Surname         string `json:"surname"`
			GivenName       string `json:"given_name"`
			Sex             string `json:"sex"`
			MatchMode       string `json:"match_mode"`
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
		args.Surname = strings.TrimSpace(args.Surname)
		if args.TreeID < 1 {
			return framework.NewToolResultError("tree_id must be a positive numeric ID"), nil
		}
		if args.Surname == "" {
			return framework.NewToolResultError("surname must not be blank"), nil
		}
		args.GivenName = strings.TrimSpace(args.GivenName)
		args.Sex = strings.TrimSpace(args.Sex)
		args.MatchMode = strings.ToLower(strings.TrimSpace(args.MatchMode))
		if args.MatchMode == "" {
			args.MatchMode = "prefix"
		}
		if args.MatchMode != "exact" && args.MatchMode != "prefix" && args.MatchMode != "fuzzy" {
			return framework.NewToolResultError("match_mode must be exact, prefix, or fuzzy"), nil
		}
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
		searchResults, err := reader.SearchPersons(genealogy.PersonSearchCriteria{
			TreeID: strconv.Itoa(args.TreeID), Surname: args.Surname, GivenName: args.GivenName, Sex: args.Sex, MatchMode: args.MatchMode,
			BirthYearMin: args.BirthYearMin, BirthYearMax: args.BirthYearMax,
			DeathYearMin: args.DeathYearMin, DeathYearMax: args.DeathYearMax,
			IncludeIndirect: args.IncludeIndirect, Limit: args.Limit, Offset: args.Offset,
		})
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		limit, offset := genealogy.NormalizePage(args.Limit, args.Offset)
		return structuredResult(searchPeopleResult(searchResults.People, searchResults.TotalCount, limit, offset), searchPeopleSummary(searchResults.People, fmt.Sprintf("Found %d people matching search %q.", searchResults.TotalCount, args.Surname)))
	}
}

func getFamilyHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID   int    `json:"tree_id"`
			FamilyID string `json:"family_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID < 1 || args.FamilyID == "" {
			return framework.NewToolResultError("tree_id must be a positive numeric ID and family_id is required"), nil
		}
		family, err := reader.GetFamily(strconv.Itoa(args.TreeID), args.FamilyID)
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
			TreeID int `json:"tree_id"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID < 1 {
			return framework.NewToolResultError("tree_id must be a positive numeric ID"), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		people, err := list(reader, strconv.Itoa(args.TreeID), args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(peopleResult(people), peopleSummary(people, fmt.Sprintf("Found %d people.", len(people))))
	}
}

func structuredResult(value any, summary string) (*framework.CallToolResult, error) {
	return framework.NewToolResultStructured(value, summary), nil
}

func getPersonByExactIDHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(ctx context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID   int    `json:"tree_id"`
			PersonID string `json:"person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID < 1 || !validPersonID(args.PersonID) {
			return framework.NewToolResultError("tree_id must be a positive numeric ID and person_id is required"), nil
		}
		person, err := reader.GetPerson(strconv.Itoa(args.TreeID), args.PersonID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(personResult(*person), personSummary(*person))
	}
}

var gedcomPersonIDPattern = regexp.MustCompile(`^I[0-9]+$`)

func validPersonID(value string) bool {
	return gedcomPersonIDPattern.MatchString(strings.TrimSpace(value))
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

func searchPeopleSummary(results []domain.PersonSearchResult, prefix string) string {
	if len(results) == 0 {
		return prefix
	}
	blocks := make([]string, 0, len(results))
	for _, result := range results {
		match := "indirect record match"
		if result.Match.DirectHit {
			match = "direct indexed name match"
		}
		block := "Result type: research lead\nMatch: " + match
		if len(result.Match.Fields) > 0 {
			block += "\nMatched fields: " + strings.Join(result.Match.Fields, ", ")
		}
		blocks = append(blocks, block+"\n"+personSummary(result.Person))
	}
	return prefix + "\n\n" + strings.Join(blocks, "\n\n")
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
		description := "- " + strconv.Itoa(tree.ID)
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
