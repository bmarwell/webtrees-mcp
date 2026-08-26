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
		framework.WithDescription("Use when you need to find people from a surname or name fragment. Do not treat indirect record matches as verified identity. Results rank direct primary, birth, maiden, married, and alternate-name matches before indirect GEDCOM-record matches, and support deterministic limit/offset pagination. Set include_indirect to true only when investigating broad record matches. Chain selected person IDs into get_person for verified detail."),
		framework.WithOutputSchema[peopleResultDTO](),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("surname", framework.Required(), framework.Description("Surname or part of a surname")),
		framework.WithBoolean("include_indirect", framework.Description("Include records where the query matches outside a name field")),
		framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10 and is capped at 100")),
		framework.WithInteger("offset", framework.Description("Number of people to skip; defaults to 0")),
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
		return fmt.Sprintf("No explicit relationship path found between %s and %s.", fromID, toID)
	}
	return fmt.Sprintf("Found an explicit relationship path from %s to %s with %d hops.", fromID, toID, len(path))
}

func searchPersonsHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID          string `json:"tree_id"`
			Surname         string `json:"surname"`
			IncludeIndirect bool   `json:"include_indirect"`
			Limit           int    `json:"limit"`
			Offset          int    `json:"offset"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" || args.Surname == "" {
			return framework.NewToolResultError("tree_id and surname are required"), nil
		}
		if args.Offset < 0 {
			return framework.NewToolResultError("offset must not be negative"), nil
		}
		results, err := reader.SearchPersons(args.TreeID, args.Surname, args.IncludeIndirect, args.Limit, args.Offset)
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
	summary := fmt.Sprintf("Person %q (%s)", name, person.ID)
	events := make([]string, 0, 2)
	if person.BirthDate != "" {
		events = append(events, "was born on "+person.BirthDate)
	} else {
		events = append(events, "has no recorded birth date")
	}
	if person.DeathDate != "" {
		events = append(events, "died on "+person.DeathDate)
	}
	summary += " " + joinPhrases(events) + "."
	if person.Occupation != "" {
		summary += " They worked as " + person.Occupation + "."
	}
	if len(person.Relatives) > 0 {
		relatives := make([]string, 0, len(person.Relatives))
		for _, relative := range person.Relatives {
			if relative.Relationship != "" {
				relatives = append(relatives, relative.PersonID+" ("+relative.Relationship+")")
			} else {
				relatives = append(relatives, relative.PersonID)
			}
		}
		summary += " Associated people: " + strings.Join(relatives, ", ") + "."
	}
	return summary
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
	return prefix + " " + strings.Join(summaries, " ")
}

func familySummary(family domain.Family) string {
	summary := fmt.Sprintf("Family %s", family.ID)
	if len(family.Parents) > 0 {
		parents := relativeIDs(family.Parents)
		summary += " has parent " + strings.Join(parents, " and ")
		if len(parents) > 1 {
			summary = fmt.Sprintf("Family %s has parents %s", family.ID, strings.Join(parents, " and "))
		}
	}
	if len(family.Children) > 0 {
		summary += "; children: " + strings.Join(relativeIDs(family.Children), ", ")
	}
	return summary + "."
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
		return "No trees found."
	}
	descriptions := make([]string, 0, len(trees))
	for _, tree := range trees {
		description := tree.ID
		if tree.Name != "" {
			description += fmt.Sprintf(" (%q)", tree.Name)
		}
		if tree.Title != "" {
			description += ": " + tree.Title
		}
		descriptions = append(descriptions, description)
	}
	return fmt.Sprintf("Found %d trees: %s.", len(trees), strings.Join(descriptions, "; "))
}
