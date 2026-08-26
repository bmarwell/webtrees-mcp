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
		framework.WithDescription("Retrieve one individual from a Webtrees tree."),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("person_id", framework.Required(), framework.Description("Individual xref, for example I1")),
	)
	s.AddTool(tool, getPersonHandler(reader))
	s.AddTool(framework.NewTool("search_persons",
		framework.WithDescription("Find people in a tree by surname."),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("surname", framework.Required(), framework.Description("Surname or part of a surname")),
	), searchPersonsHandler(reader))
	s.AddTool(framework.NewTool("get_family",
		framework.WithDescription("Retrieve the parent and child links for one family."),
		framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
		framework.WithString("family_id", framework.Required(), framework.Description("Family xref, for example F1")),
	), getFamilyHandler(reader))
	s.AddTool(framework.NewTool("list_tree_ids",
		framework.WithDescription("List available Webtrees trees.")), listTreesHandler(reader))
	for _, spec := range []struct {
		name string
		desc string
		fn   func(genealogy.Repository, string, int) ([]domain.Person, error)
	}{
		{"list_recently_born", "List people ordered by the year of their birth.", func(r genealogy.Repository, treeID string, limit int) ([]domain.Person, error) {
			return r.ListRecentlyBorn(treeID, limit)
		}},
		{"list_recently_deceased", "List people ordered by the year of their death.", func(r genealogy.Repository, treeID string, limit int) ([]domain.Person, error) {
			return r.ListRecentlyDeceased(treeID, limit)
		}},
	} {
		s.AddTool(framework.NewTool(spec.name, framework.WithDescription(spec.desc),
			framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
			framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10")),
		), listPeopleHandler(reader, spec.fn))
	}
}

func searchPersonsHandler(reader genealogy.Repository) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID  string `json:"tree_id"`
			Surname string `json:"surname"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" || args.Surname == "" {
			return framework.NewToolResultError("tree_id and surname are required"), nil
		}
		people, err := reader.SearchPersons(args.TreeID, args.Surname)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(personOutputs(people), peopleSummary(people, fmt.Sprintf("Found %d people matching surname %q.", len(people), args.Surname)))
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
	return func(context.Context, framework.CallToolRequest) (*framework.CallToolResult, error) {
		trees, err := reader.ListTrees()
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(treeOutputs(trees), treesSummary(trees))
	}
}

func listPeopleHandler(reader genealogy.Repository, list func(genealogy.Repository, string, int) ([]domain.Person, error)) server.ToolHandlerFunc {
	return func(_ context.Context, request framework.CallToolRequest) (*framework.CallToolResult, error) {
		var args struct {
			TreeID string `json:"tree_id"`
			Limit  int    `json:"limit"`
		}
		if err := request.BindArguments(&args); err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		if args.TreeID == "" {
			return framework.NewToolResultError("tree_id is required"), nil
		}
		people, err := list(reader, args.TreeID, args.Limit)
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return structuredResult(personOutputs(people), peopleSummary(people, fmt.Sprintf("Found %d people.", len(people))))
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
