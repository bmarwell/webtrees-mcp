package mcp

import (
	"context"
	"encoding/json"

	"webtrees-mcp/internal/db"
	"webtrees-mcp/internal/model"

	framework "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(s *server.MCPServer, reader *db.Reader) {
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
		fn   func(*db.Reader, string, int) ([]model.PersonDTO, error)
	}{
		{"list_recently_born", "List people ordered by the year of their birth.", (*db.Reader).ListRecentlyBorn},
		{"list_recently_deceased", "List people ordered by the year of their death.", (*db.Reader).ListRecentlyDeceased},
	} {
		s.AddTool(framework.NewTool(spec.name, framework.WithDescription(spec.desc),
			framework.WithString("tree_id", framework.Required(), framework.Description("Webtrees tree ID")),
			framework.WithInteger("limit", framework.Description("Maximum number of people; defaults to 10")),
		), listPeopleHandler(reader, spec.fn))
	}
}

func searchPersonsHandler(reader *db.Reader) server.ToolHandlerFunc {
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
		return jsonResult(people)
	}
}

func getFamilyHandler(reader *db.Reader) server.ToolHandlerFunc {
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
		return jsonResult(family)
	}
}

func listTreesHandler(reader *db.Reader) server.ToolHandlerFunc {
	return func(context.Context, framework.CallToolRequest) (*framework.CallToolResult, error) {
		trees, err := reader.ListTrees()
		if err != nil {
			return framework.NewToolResultError(err.Error()), nil
		}
		return jsonResult(trees)
	}
}

func listPeopleHandler(reader *db.Reader, list func(*db.Reader, string, int) ([]model.PersonDTO, error)) server.ToolHandlerFunc {
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
		return jsonResult(people)
	}
}

func jsonResult(value any) (*framework.CallToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return framework.NewToolResultText(string(data)), nil
}

func getPersonHandler(reader *db.Reader) server.ToolHandlerFunc {
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
		return jsonResult(person)
	}
}
