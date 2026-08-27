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

func newReadOnlyTool(name string, options ...framework.ToolOption) framework.Tool {
	readOnly := []framework.ToolOption{
		framework.WithReadOnlyHintAnnotation(true),
		framework.WithDestructiveHintAnnotation(false),
		framework.WithIdempotentHintAnnotation(true),
		framework.WithOpenWorldHintAnnotation(false),
	}
	return framework.NewTool(name, append(readOnly, options...)...)
}

func RegisterTools(s *server.MCPServer, reader genealogy.Repository, treeID string) {
	tool := newReadOnlyTool("get_person_by_exact_id",
		framework.WithDescription("Use when you already have an exact GEDCOM person_id such as I123 and need verified details. Do not pass a person's name here and do not use for name searches; call search_person_by_name first. Chain the returned person_id here. Read content as the complete factual result; do not infer omitted facts."),
		framework.WithOutputSchema[personResultDTO](),
		framework.WithString("person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name; search by name first.")),
	)
	s.AddTool(tool, getPersonByExactIDHandler(reader, treeID))
	s.AddTool(newReadOnlyTool("search_person_by_name",
		framework.WithDescription("Use when you have a person's name and need a research lead. Required: surname; optional: given_name, sex, match_mode (exact, prefix, or fuzzy; default prefix), birth_year_min/max, death_year_min/max, limit 1-100 (default 10), offset 0-10000 (default 0). The tree is configured at server startup. Do not pass a name to get_person_by_exact_id or treat results as verified facts; fuzzy mode is bounded and may miss candidates. Chain a returned person_id into get_person_by_exact_id. Read content as the factual lead record."),
		framework.WithOutputSchema[searchPeopleResultDTO](),
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
	), searchPersonByNameHandler(reader, treeID))
	s.AddTool(newReadOnlyTool("get_family_by_exact_id",
		framework.WithDescription("Use when you have an exact family_id and need its links and family evidence. Do not pass a person's name or infer relationships from surnames. Children include available names, birth years, and sex. Chain returned person_id values into get_person_by_exact_id."),
		framework.WithOutputSchema[familyOutputDTO](),
		framework.WithString("family_id", framework.Required(), framework.Description("Family xref, for example F1")),
	), getFamilyHandler(reader, treeID))
	s.AddTool(newReadOnlyTool("relationship_path",
		framework.WithDescription("Use when checking how two individuals are related, connected, or share a relationship path. PRIMARY TOOL: ALWAYS use this first when both person IDs are known. Pass exact IDs in from_person_id and to_person_id. Do not manually traverse families line by line when both IDs are available. The result uses only explicit family links; verify returned evidence with get_person_by_exact_id or get_family_by_exact_id. Chain returned IDs into those tools."),
		framework.WithOutputSchema[relationshipPathResultDTO](),
		framework.WithString("from_person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name.")),
		framework.WithString("to_person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I456. Do not pass a person's name.")),
	), relationshipPathHandler(reader, treeID))
	lineageTools := []struct {
		name        string
		direction   string
		description string
	}{
		{"get_ancestors", "ancestors", "Use when you need the direct ancestor line for a known person. Do not infer ancestors from surnames or incomplete family links; traversal is bounded by depth and limit. Chain returned person_id and via_family_id values into get_person_by_exact_id and get_family."},
		{"get_descendants", "descendants", "Use when you need the direct descendant line for a known person. Do not infer descendants from surnames or incomplete family links; traversal is bounded by depth and limit. Chain returned person_id and via_family_id values into get_person_by_exact_id and get_family."},
	}
	for _, spec := range lineageTools {
		s.AddTool(newReadOnlyTool(spec.name,
			framework.WithDescription(spec.description), framework.WithOutputSchema[lineageResultDTO](),
			framework.WithString("person_id", framework.Required(), framework.Pattern(`^I[0-9]+$`), framework.Description("Exact numeric GEDCOM individual xref, for example I123. Do not pass a person's name; search by name first.")),
			framework.WithInteger("max_depth", framework.DefaultNumber(genealogy.DefaultLineageDepth), framework.Min(1), framework.Max(genealogy.MaxLineageDepth), framework.Description("Maximum number of generations to traverse")),
			framework.WithInteger("limit", framework.DefaultNumber(genealogy.DefaultLineageLimit), framework.Min(1), framework.Max(genealogy.MaxLineageLimit), framework.Description("Maximum people to return")),
		), lineageHandler(reader, treeID, spec.direction))
	}
	s.AddTool(newReadOnlyTool("search_events",
		framework.WithDescription("Use when searching indexed individual events by type, date range, or place. Do not treat an indexed year as more precise than the source date; results are leads, not proof. Chain returned person_id values into get_person_by_exact_id for full evidence."),
		framework.WithOutputSchema[eventSearchResultsDTO](),
		framework.WithString("event_type", framework.Description("GEDCOM event type, for example BIRT, DEAT, or MARR")),
		framework.WithString("from_date", framework.Description("Inclusive lower date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("to_date", framework.Description("Inclusive upper date; preferred YYYY-MM-DD, also YYYY, YYYY-MM, or common GEDCOM/text dates")),
		framework.WithString("place", framework.Description("Case-insensitive place substring")),
		framework.WithInteger("limit", framework.Description("Maximum events; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of matching events to skip; defaults to 0")),
	), searchEventsHandler(reader, treeID))
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
		s.AddTool(newReadOnlyTool(spec.name, framework.WithDescription(spec.desc),
			framework.WithOutputSchema[peopleResultDTO](),
			framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10 and is capped at 100")),
			framework.WithInteger("offset", framework.Description("Number of people to skip; defaults to 0")),
		), listPeopleHandler(reader, treeID, spec.fn))
	}
}

func searchEventsHandler(reader genealogy.Repository, treeID string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
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
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		events, err := reader.SearchEvents(treeID, args.EventType, args.FromDate, args.ToDate, args.Place, args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		output := eventSearchResults(events)
		output.AIContext = aiContextDTO{Hint: "Search results are research leads; verify each returned person with get_person_by_exact_id.", NextAction: "Call get_person_by_exact_id for a returned person_id."}
		return structuredResult(output, eventSearchSummary(events)+"\n\nNext action: call get_person_by_exact_id for a returned person_id to verify the event.")
	}
}

func relationshipPathHandler(reader genealogy.Repository, treeID string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			FromPersonID string `json:"from_person_id"`
			ToPersonID   string `json:"to_person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if !validPersonID(args.FromPersonID) || !validPersonID(args.ToPersonID) {
			return framework.NewToolResultError("from_person_id and to_person_id must be exact GEDCOM IDs such as I123"), nil
		}
		path, found, err := genealogy.FindRelationshipPath(reader, treeID, args.FromPersonID, args.ToPersonID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		people := make(map[string]personResultDTO)
		domainPeople := make(map[string]domain.Person)
		ids := []string{args.FromPersonID, args.ToPersonID}
		for _, step := range path {
			ids = append(ids, step.FromPersonID, step.ToPersonID)
		}
		for _, id := range ids {
			if _, exists := domainPeople[id]; exists {
				continue
			}
			person, err := reader.GetPerson(treeID, id)
			if err != nil || person == nil {
				continue
			}
			families, related := resolveFamilyLinks(reader, treeID, *person)
			domainPeople[id] = *person
			people[id] = enrichedPersonResult(*person, families, related)
		}
		output := relationshipPathResultWithPeople(args.FromPersonID, args.ToPersonID, found, path, people)
		output.AIContext = aiContextDTO{Hint: "Resolved person details are included for path nodes. More complete or authoritative person data is available from get_person_by_exact_id.", NextAction: "Call get_person_by_exact_id for a path person_id when additional detail or source evidence is needed."}
		return structuredResult(output, relationshipPathSummaryWithPeople(args.FromPersonID, args.ToPersonID, found, path, domainPeople)+"\nNext action: call get_person_by_exact_id for a path person_id when additional detail or source evidence is needed.")
	}
}

func relationshipPathSummaryWithPeople(fromID, toID string, found bool, path []domain.RelationshipPathStep, people map[string]domain.Person) string {
	lines := []string{relationshipPathSummary(fromID, toID, found, path)}
	if !found || len(people) == 0 {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Path person details:")
	seen := make(map[string]bool)
	for _, step := range path {
		for _, id := range []string{step.FromPersonID, step.ToPersonID} {
			if seen[id] {
				continue
			}
			if person, ok := people[id]; ok {
				lines = append(lines, personSummary(person))
				seen[id] = true
			}
		}
	}
	return strings.Join(lines, "\n\n")
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

func lineageHandler(reader genealogy.Repository, treeID, direction string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			PersonID string `json:"person_id"`
			MaxDepth int    `json:"max_depth"`
			Limit    int    `json:"limit"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if !validPersonID(args.PersonID) {
			return framework.NewToolResultError("person_id must be an exact GEDCOM ID such as I123"), nil
		}
		if args.MaxDepth < 0 || args.MaxDepth > genealogy.MaxLineageDepth || args.Limit < 0 || args.Limit > genealogy.MaxLineageLimit {
			return framework.NewToolResultError(fmt.Sprintf("max_depth must be 1-%d and limit must be 1-%d (or omitted)", genealogy.MaxLineageDepth, genealogy.MaxLineageLimit)), nil
		}
		rootID := strings.TrimSpace(args.PersonID)
		var result domain.LineageResult
		var err error
		if direction == "ancestors" {
			result, err = genealogy.FindAncestors(reader, treeID, rootID, args.MaxDepth, args.Limit)
		} else {
			result, err = genealogy.FindDescendants(reader, treeID, rootID, args.MaxDepth, args.Limit)
		}
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		output := lineageResult(result)
		if result.Truncated {
			output.AIContext = aiContextDTO{Hint: "The lineage is bounded and truncated at the configured limit.", NextAction: "Repeat with a larger limit or max_depth if more generations are needed."}
		} else {
			output.AIContext = aiContextDTO{Hint: "The result contains only explicit family-link traversal.", NextAction: "Call get_person_by_exact_id for a returned person_id when detailed evidence is needed."}
		}
		return structuredResult(output, lineageSummary(result)+"\nNext action: "+output.AIContext.NextAction)
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

func searchPersonByNameHandler(reader genealogy.Repository, treeID string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
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
			TreeID: treeID, Surname: args.Surname, GivenName: args.GivenName, Sex: args.Sex, MatchMode: args.MatchMode,
			BirthYearMin: args.BirthYearMin, BirthYearMax: args.BirthYearMax,
			DeathYearMin: args.DeathYearMin, DeathYearMax: args.DeathYearMax,
			IncludeIndirect: args.IncludeIndirect, Limit: args.Limit, Offset: args.Offset,
		})
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		limit, offset := genealogy.NormalizePage(args.Limit, args.Offset)
		output := searchPeopleResult(searchResults.People, searchResults.TotalCount, limit, offset)
		for i, result := range searchResults.People {
			families, people := resolveFamilyLinks(reader, treeID, result.Person)
			output.People[i] = enrichedPersonResult(result.Person, families, people)
		}
		output.AIContext = searchAIContext(len(searchResults.People), output.HasMore, offset, limit)
		return structuredResult(output, enrichedSearchPeopleSummary(searchResults.People, reader, treeID, fmt.Sprintf("Found %d people matching search %q.", searchResults.TotalCount, args.Surname))+"\n\nNext action: "+output.AIContext.NextAction)
	}
}

func searchAIContext(resultCount int, hasMore bool, offset, limit int) aiContextDTO {
	if resultCount >= 2 {
		return aiContextDTO{Hint: "If you are trying to determine how two found individuals are connected, call relationship_path directly instead of inspecting family entries manually.", NextAction: "Call relationship_path with two returned person_id values."}
	}
	if hasMore {
		return aiContextDTO{Hint: "The result is paginated; use the same search with the next offset.", NextAction: fmt.Sprintf("Call search_person_by_name again with offset=%d.", offset+limit)}
	}
	return aiContextDTO{Hint: "Search results are research leads, not verified identities.", NextAction: "Call get_person_by_exact_id for a selected person_id."}
}

func getFamilyHandler(reader genealogy.Repository, treeID string) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			FamilyID string `json:"family_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.FamilyID == "" {
			return framework.NewToolResultError("family_id is required"), nil
		}
		family, err := reader.GetFamily(treeID, args.FamilyID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		children := make(map[string]domain.Person, len(family.Children)+len(family.Parents))
		participants := append(append([]domain.Relative{}, family.Children...), family.Parents...)
		for _, child := range participants {
			person, err := reader.GetPerson(treeID, child.PersonID)
			if err == nil && person != nil {
				children[child.PersonID] = *person
			}
		}
		output := familyOutput(*family, children)
		output.AIContext = aiContextDTO{Hint: "Family links are explicit records; resolved names remain evidence from the active tree.", NextAction: "Call get_person_by_exact_id for a parent or child person_id to inspect full details."}
		return structuredResult(output, familySummary(*family, children)+"\nNext action: call get_person_by_exact_id for a parent or child person_id to inspect full details.")
	}
}

func listPeopleHandler(reader genealogy.Repository, treeID string, list func(genealogy.Repository, string, int, int) ([]domain.Person, error)) server.ToolHandlerFunc {
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
		people, err := list(reader, treeID, args.Limit, args.Offset)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		output := peopleResult(people)
		output.AIContext = aiContextDTO{Hint: "This ordered list is a research lead and may be incomplete because of pagination.", NextAction: "Call get_person_by_exact_id for a selected person_id."}
		return structuredResult(output, peopleSummary(people, fmt.Sprintf("Found %d people.", len(people)))+"\n\nNext action: call get_person_by_exact_id for a selected person_id.")
	}
}

func structuredResult(value any, summary string) (*framework.CallToolResult, error) {
	return framework.NewToolResultStructured(value, summary), nil
}

func getPersonByExactIDHandler(reader genealogy.Repository, treeID string) server.ToolHandlerFunc {
	return func(ctx context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			PersonID string `json:"person_id"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if !validPersonID(args.PersonID) {
			return framework.NewToolResultError("person_id must be an exact GEDCOM ID such as I123"), nil
		}
		person, err := reader.GetPerson(treeID, args.PersonID)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		families, people := resolveFamilyLinks(reader, treeID, *person)
		output := enrichedPersonResult(*person, families, people)
		output.AIContext = personAIContext(*person, families)
		return structuredResult(output, enrichedPersonSummary(*person, families, people)+"\nNext action: "+output.AIContext.NextAction)
	}
}

var gedcomPersonIDPattern = regexp.MustCompile(`^I[0-9]+$`)

func validPersonID(value string) bool {
	return gedcomPersonIDPattern.MatchString(strings.TrimSpace(value))
}

func resolveFamilyLinks(reader genealogy.Repository, treeID string, person domain.Person) (map[string]domain.Family, map[string]domain.Person) {
	families := make(map[string]domain.Family, len(person.FamilyLinks))
	people := make(map[string]domain.Person)
	for _, link := range person.FamilyLinks {
		family, err := reader.GetFamily(treeID, link.FamilyID)
		if err != nil || family == nil {
			continue
		}
		families[link.FamilyID] = *family
		ids := family.Parents
		if link.Role == "spouse" {
			ids = append(ids, family.Children...)
		}
		for _, relative := range ids {
			if _, exists := people[relative.PersonID]; exists {
				continue
			}
			related, err := reader.GetPerson(treeID, relative.PersonID)
			if err == nil && related != nil {
				people[relative.PersonID] = *related
			}
		}
	}
	return families, people
}

func personAIContext(person domain.Person, families map[string]domain.Family) aiContextDTO {
	parents := make([]string, 0)
	spouses := make([]string, 0)
	for _, link := range person.FamilyLinks {
		family, ok := families[link.FamilyID]
		if !ok {
			continue
		}
		if link.Role == "child" {
			for _, parent := range family.Parents {
				if parent.PersonID != person.ID {
					parents = append(parents, parent.PersonID)
				}
			}
		} else if link.Role == "spouse" {
			for _, spouse := range family.Parents {
				if spouse.PersonID != person.ID {
					spouses = append(spouses, spouse.PersonID)
				}
			}
		}
	}
	if len(parents) > 0 {
		return aiContextDTO{ParentsFound: parents, SpousesFound: spouses, Hint: "Parent IDs can be used to investigate siblings through families where they are listed as parents.", NextAction: "Call get_family_by_exact_id for a family_id or get_person_by_exact_id for a selected person_id."}
	}
	if len(spouses) > 0 {
		return aiContextDTO{SpousesFound: spouses, Hint: "A spouse ID can be used to inspect the linked family and its children.", NextAction: "Call get_family_by_exact_id for a family_id or get_person_by_exact_id for the spouse person_id."}
	}
	return aiContextDTO{Hint: "No resolvable parent or spouse family links were found in this result.", NextAction: "Use search_person_by_name if another person must be located."}
}

func personSummary(person domain.Person) string {
	return enrichedPersonSummary(person, nil, nil)
}

func enrichedPersonSummary(person domain.Person, families map[string]domain.Family, people map[string]domain.Person) string {
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
			description := "- family_id=" + link.FamilyID + "; role=" + link.Role
			if family, ok := families[link.FamilyID]; ok {
				description += "; family_name=" + valueOrNotRecorded(familyName(family, people))
				if link.Role == "child" {
					for _, parent := range family.Parents {
						description += fmt.Sprintf("; parent=%s (%s; role=%s)", parent.PersonID, valueOrNotRecorded(displayName(people[parent.PersonID])), parentRole(people[parent.PersonID]))
					}
				} else if link.Role == "spouse" {
					description += fmt.Sprintf("; children_count=%d", len(family.Children))
					for _, spouse := range family.Parents {
						if spouse.PersonID != person.ID {
							description += fmt.Sprintf("; spouse=%s (%s)", spouse.PersonID, valueOrNotRecorded(displayName(people[spouse.PersonID])))
							break
						}
					}
				}
			}
			lines = append(lines, description)
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

func enrichedSearchPeopleSummary(results []domain.PersonSearchResult, reader genealogy.Repository, treeID, prefix string) string {
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
		families, people := resolveFamilyLinks(reader, treeID, result.Person)
		blocks = append(blocks, block+"\n"+enrichedPersonSummary(result.Person, families, people))
	}
	return prefix + "\n\n" + strings.Join(blocks, "\n\n")
}

func birthYear(value string) *int {
	for _, field := range strings.Fields(value) {
		field = strings.Trim(field, "(),.-")
		if len(field) == 4 {
			year, err := strconv.Atoi(field)
			if err == nil && year > 0 {
				return &year
			}
		}
	}
	return nil
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

func familySummary(family domain.Family, childPeople ...map[string]domain.Person) string {
	lines := []string{"Family ID: " + family.ID}
	if len(childPeople) > 0 {
		lines = append(lines, "Family name: "+valueOrNotRecorded(familyName(family, childPeople[0])))
	}
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
			description := "- " + child.PersonID
			if len(childPeople) > 0 {
				description = "- person_id=" + child.PersonID
				if person, ok := childPeople[0][child.PersonID]; ok {
					description += "; name=" + valueOrNotRecorded(displayName(person))
					description += "; birth_year=" + valueOrNotRecorded(formatBirthYear(person.BirthDate))
					description += "; sex=" + valueOrNotRecorded(person.Sex)
				}
			}
			lines = append(lines, description)
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

func formatBirthYear(value string) string {
	year := birthYear(value)
	if year == nil {
		return ""
	}
	return strconv.Itoa(*year)
}

func relativeIDs(relatives []domain.Relative) []string {
	ids := make([]string, 0, len(relatives))
	for _, relative := range relatives {
		ids = append(ids, relative.PersonID)
	}
	return ids
}
