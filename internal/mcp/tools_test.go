package mcp

import (
	"context"
	"reflect"
	"strings"
	"testing"

	framework "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"webtrees-mcp/internal/domain"
)

func TestStructuredResultUsesStructuredContent(t *testing.T) {
	payload := map[string]string{"name": "Ada"}
	result, err := structuredResult(payload, "Found Ada.")
	if err != nil {
		t.Fatalf("jsonResult returned an error: %v", err)
	}
	if result.StructuredContent == nil {
		t.Fatal("structured content should be present")
	}
	if !reflect.DeepEqual(result.StructuredContent, payload) {
		t.Errorf("structured content = %#v, want %#v", result.StructuredContent, payload)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one content item, got %d", len(result.Content))
	}
	content, ok := result.Content[0].(framework.TextContent)
	if !ok || content.Text != "Found Ada." {
		t.Fatalf("unexpected content: %#v", result.Content[0])
	}
}

func TestRegisteredToolsPublishGuidanceAndOutputSchemas(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0")
	RegisterTools(mcpServer, nil)
	for _, name := range []string{"get_person", "search_persons", "get_family", "relationship_path", "search_events", "list_tree_ids", "list_recently_born", "list_recently_deceased"} {
		tool := mcpServer.GetTool(name)
		if tool == nil {
			t.Fatalf("tool %q was not registered", name)
		}
		for _, phrase := range []string{"Use when", "Do not", "chain"} {
			if !strings.Contains(strings.ToLower(tool.Tool.Description), strings.ToLower(phrase)) {
				t.Errorf("tool %q lacks %q selection guidance: %q", name, phrase, tool.Tool.Description)
			}
		}
		if tool.Tool.OutputSchema.Type != "object" {
			t.Errorf("tool %q output schema type = %q, want object", name, tool.Tool.OutputSchema.Type)
		}
		if name != "list_tree_ids" {
			treeSchema, ok := tool.Tool.InputSchema.Properties["tree_id"].(map[string]any)
			if !ok || treeSchema["type"] != "integer" || treeSchema["minimum"] != 1 {
				t.Errorf("tool %q must declare positive numeric tree_id: %#v", name, tool.Tool.InputSchema.Properties["tree_id"])
			}
		}
	}
	searchTool := mcpServer.GetTool("search_persons")
	for _, field := range []string{"people", "total_count", "has_more", "limit", "offset"} {
		if _, ok := searchTool.Tool.OutputSchema.Properties[field]; !ok {
			t.Errorf("search_persons output schema lacks %s", field)
		}
	}
	if !strings.Contains(strings.ToLower(searchTool.Tool.Description), "read content") {
		t.Errorf("search_persons description lacks content guidance: %q", searchTool.Tool.Description)
	}
	if _, ok := searchTool.Tool.InputSchema.Properties["include_indirect"]; !ok {
		t.Error("search_persons lacks include_indirect input metadata")
	}
	matchModeSchema, ok := searchTool.Tool.InputSchema.Properties["match_mode"].(map[string]any)
	if !ok || matchModeSchema["default"] != "prefix" || !reflect.DeepEqual(matchModeSchema["enum"], []string{"exact", "prefix", "fuzzy"}) {
		t.Errorf("unexpected search_persons match_mode schema: %#v", searchTool.Tool.InputSchema.Properties["match_mode"])
	}
	limitSchema, ok := searchTool.Tool.InputSchema.Properties["limit"].(map[string]any)
	if !ok || limitSchema["default"] != 10 || limitSchema["minimum"] != 1 || limitSchema["maximum"] != 100 {
		t.Errorf("unexpected search_persons limit schema: %#v", searchTool.Tool.InputSchema.Properties["limit"])
	}
	offsetSchema, ok := searchTool.Tool.InputSchema.Properties["offset"].(map[string]any)
	if !ok || offsetSchema["default"] != 0 || offsetSchema["minimum"] != 0 || offsetSchema["maximum"] != 10000 {
		t.Errorf("unexpected search_persons offset schema: %#v", searchTool.Tool.InputSchema.Properties["offset"])
	}
	for _, name := range []string{"search_persons", "list_tree_ids", "list_recently_born", "list_recently_deceased"} {
		tool := mcpServer.GetTool(name)
		for _, argument := range []string{"limit", "offset"} {
			if _, ok := tool.Tool.InputSchema.Properties[argument]; !ok {
				t.Errorf("%s lacks %s input metadata", name, argument)
			}
		}
	}
	for _, argument := range []string{"given_name", "sex", "birth_year_min", "birth_year_max", "death_year_min", "death_year_max"} {
		if _, ok := searchTool.Tool.InputSchema.Properties[argument]; !ok {
			t.Errorf("search_persons lacks %s input metadata", argument)
		}
	}
}

func TestSearchPersonsRejectsInvalidInput(t *testing.T) {
	handler := searchPersonsHandler(nil)
	for _, test := range []struct {
		name string
		args map[string]any
	}{
		{name: "blank tree", args: map[string]any{"tree_id": " ", "surname": "Example"}},
		{name: "blank surname", args: map[string]any{"tree_id": "tree", "surname": "\t"}},
		{name: "negative limit", args: map[string]any{"tree_id": "tree", "surname": "Example", "limit": -1}},
		{name: "limit too large", args: map[string]any{"tree_id": "tree", "surname": "Example", "limit": 101}},
		{name: "negative offset", args: map[string]any{"tree_id": "tree", "surname": "Example", "offset": -1}},
		{name: "offset too large", args: map[string]any{"tree_id": "tree", "surname": "Example", "offset": 10001}},
		{name: "unknown match mode", args: map[string]any{"tree_id": "tree", "surname": "Example", "match_mode": "contains"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := handler(context.Background(), framework.CallToolRequest{
				Params: framework.CallToolParams{Arguments: test.args},
			})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if result == nil || !result.IsError {
				t.Fatalf("expected an error result, got %#v", result)
			}
		})
	}
}

func TestPersonResultMapsDomainToTransportDTO(t *testing.T) {
	result := personResult(domain.Person{
		ID:        "I44",
		Name:      domain.Name{Given: "Ada", Surname: "Mayer"},
		BirthDate: "1982",
		Relatives: []domain.Relative{{PersonID: "I45", Relationship: "spouse"}},
		Events:    []domain.Event{{Type: "birt", Date: &domain.Date{Raw: "ABT 1982", Precision: "about"}, Place: "Stadthagen"}},
	})
	if result.ID != "I44" || result.Name.Given != "Ada" || result.BirthDate != "1982" {
		t.Fatalf("unexpected mapped person: %+v", result)
	}
	if len(result.Relatives) != 1 || result.Relatives[0].PersonID != "I45" {
		t.Fatalf("unexpected mapped relatives: %+v", result.Relatives)
	}
	if len(result.Events) != 1 || result.Events[0].Date == nil || result.Events[0].Date.Precision != "about" || result.Events[0].Place != "Stadthagen" {
		t.Fatalf("unexpected mapped events: %+v", result.Events)
	}
}

func TestCollectionResultsUseObjectShapes(t *testing.T) {
	people := peopleResult([]domain.Person{{ID: "I44"}})
	if len(people.People) != 1 || people.People[0].ID != "I44" {
		t.Fatalf("unexpected people result: %+v", people)
	}
	trees := treesResult([]domain.Tree{{ID: 42}})
	if len(trees.Trees) != 1 || trees.Trees[0].ID != 42 {
		t.Fatalf("unexpected trees result: %+v", trees)
	}

	result, err := structuredResult(people, "Found 1 person.")
	if err != nil {
		t.Fatalf("structuredResult returned an error: %v", err)
	}
	if _, ok := result.StructuredContent.(peopleResultDTO); !ok {
		t.Fatalf("collection structured content should be an object, got %T", result.StructuredContent)
	}
}

func TestSearchPeopleResultIncludesPaginationMetadata(t *testing.T) {
	result := searchPeopleResult([]domain.PersonSearchResult{{Person: domain.Person{ID: "I1"}}}, 3, 1, 1)
	if len(result.People) != 1 || result.TotalCount != 3 || !result.HasMore || result.Limit != 1 || result.Offset != 1 {
		t.Fatalf("unexpected search pagination result: %+v", result)
	}
	if final := searchPeopleResult(nil, 0, 10, 0); final.HasMore {
		t.Fatal("last page must not report more results")
	}
}

func TestSearchPersonResultIncludesMatchMetadata(t *testing.T) {
	result := searchPersonResult(domain.PersonSearchResult{
		Person: domain.Person{ID: "I44"},
		Match:  domain.SearchMatch{DirectHit: false, Fields: []string{"gedcom_record"}},
	})
	if result.Match == nil || result.Match.DirectHit || len(result.Match.Fields) != 1 || result.Match.Fields[0] != "gedcom_record" {
		t.Fatalf("unexpected search match metadata: %+v", result.Match)
	}
}

func TestSearchPeopleSummaryMarksLeadsAndMatchType(t *testing.T) {
	got := searchPeopleSummary([]domain.PersonSearchResult{{
		Person: domain.Person{ID: "I7", Name: domain.Name{Given: "Casey", Surname: "Example"}},
		Match:  domain.SearchMatch{DirectHit: true, Fields: []string{"name"}},
	}}, "Found 1 person.")
	if !strings.Contains(got, "Result type: research lead") || !strings.Contains(got, "Match: direct indexed name match") || !strings.Contains(got, "Matched fields: name") {
		t.Errorf("search summary lacks lead metadata: %q", got)
	}
}

func TestPersonSummaryAddsOptionalPhrases(t *testing.T) {
	tests := []struct {
		name   string
		person domain.Person
		want   string
	}{
		{
			name: "complete",
			person: domain.Person{
				ID: "I44", Name: domain.Name{Given: "Ada", Surname: "Mayer"},
				BirthDate: "1982", DeathDate: "2026", Occupation: "Carpenter",
				Relatives: []domain.Relative{{PersonID: "I45", Relationship: "spouse"}},
			},
			want: "Person ID: I44\nName: Ada Mayer\nSex: not recorded\nBirth date: 1982\nDeath date: 2026\nOccupation: Carpenter\nEvents:\n- BIRT:\n  Date: 1982\n  Place: not recorded\n  Value: not recorded\n  Notes: none\n  Sources: none\n- DEAT:\n  Date: 2026\n  Place: not recorded\n  Value: not recorded\n  Notes: none\n  Sources: none\n- OCCU:\n  Date: not recorded\n  Place: not recorded\n  Value: Carpenter\n  Notes: none\n  Sources: none\nAlternate names: none\nRelatives:\n- I45 (spouse)\nFamily links: none\nNotes: none\nSources: none",
		},
		{
			name:   "missing optional fields",
			person: domain.Person{ID: "I44", Name: domain.Name{Given: "Ada"}},
			want:   "Person ID: I44\nName: Ada\nSex: not recorded\nBirth date: not recorded\nDeath date: not recorded\nOccupation: not recorded\nEvents: none\nAlternate names: none\nRelatives: none\nFamily links: none\nNotes: none\nSources: none",
		},
		{
			name: "death without birth",
			person: domain.Person{
				ID: "I45", Name: domain.Name{Given: "John", Surname: "Doe"}, DeathDate: "2026",
			},
			want: "Person ID: I45\nName: John Doe\nSex: not recorded\nBirth date: not recorded\nDeath date: 2026\nOccupation: not recorded\nEvents:\n- DEAT:\n  Date: 2026\n  Place: not recorded\n  Value: not recorded\n  Notes: none\n  Sources: none\nAlternate names: none\nRelatives: none\nFamily links: none\nNotes: none\nSources: none",
		},
		{
			name: "relative without relationship",
			person: domain.Person{
				ID: "I46", Relatives: []domain.Relative{{PersonID: "I44"}},
			},
			want: "Person ID: I46\nName: Unknown person\nSex: not recorded\nBirth date: not recorded\nDeath date: not recorded\nOccupation: not recorded\nEvents: none\nAlternate names: none\nRelatives:\n- I44\nFamily links: none\nNotes: none\nSources: none",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := personSummary(test.person); got != test.want {
				t.Errorf("personSummary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPersonSummaryListsEventsWithTypeAndPlace(t *testing.T) {
	got := personSummary(domain.Person{
		ID: "I7", Name: domain.Name{Given: "Casey", Surname: "Example"},
		Events: []domain.Event{
			{Type: "birt", Date: &domain.Date{Raw: "1 JAN 1900"}, Place: "Exampletown", Value: "born at home", Notes: []string{"Family Bible"}, Sources: []domain.Source{{ID: "S1", Title: "Birth record"}}},
			{Type: "even", Value: "joined an association"},
		},
	})
	want := "Person ID: I7\nName: Casey Example\nSex: not recorded\nBirth date: not recorded\nDeath date: not recorded\nOccupation: not recorded\nEvents:\n- BIRT:\n  Date: 1 JAN 1900\n  Place: Exampletown\n  Value: born at home\n  Notes:\n  - Family Bible\n  Sources:\n  - S1 (Birth record)\n- EVEN:\n  Date: not recorded\n  Place: not recorded\n  Value: joined an association\n  Notes: none\n  Sources: none\nAlternate names: none\nRelatives: none\nFamily links: none\nNotes: none\nSources: none"
	if got != want {
		t.Errorf("personSummary() = %q, want %q", got, want)
	}
}

func TestFamilyAndTreeSummariesHandleOptionalData(t *testing.T) {
	if got := familySummary(domain.Family{ID: "F1"}); got != "Family ID: F1\nParents: none\nChildren: none\nEvents: none\nNotes: none\nSources: none" {
		t.Errorf("familySummary() = %q", got)
	}
	if got := familySummary(domain.Family{
		ID:       "F1",
		Parents:  []domain.Relative{{PersonID: "I1"}, {PersonID: "I2"}},
		Children: []domain.Relative{{PersonID: "I3"}, {PersonID: "I4"}},
	}); got != "Family ID: F1\nParents:\n- I1\n- I2\nChildren:\n- I3\n- I4\nEvents: none\nNotes: none\nSources: none" {
		t.Errorf("familySummary() = %q", got)
	}
	if got := treesSummary(nil); got != "Trees: none found." {
		t.Errorf("treesSummary(nil) = %q", got)
	}
}

func TestTextSummariesUseExplicitBlocksForCollections(t *testing.T) {
	people := peopleSummary([]domain.Person{{ID: "I1", Name: domain.Name{Given: "Casey"}}, {ID: "I2"}}, "Found 2 people.")
	if !strings.Contains(people, "Found 2 people.\n\nPerson ID: I1") || !strings.Contains(people, "\n\nPerson ID: I2") {
		t.Errorf("people summary is not block-oriented: %q", people)
	}

	events := eventSearchSummary([]domain.EventSearchResult{{PersonID: "I1", Type: "birt", Date: "1900", Place: "Exampletown"}})
	if events != "Events: 1\n- Person: I1; Type: BIRT; Date: 1900; Place: Exampletown" {
		t.Errorf("event search summary = %q", events)
	}

	path := relationshipPathSummary("I1", "I2", true, []domain.RelationshipPathStep{{FromPersonID: "I1", ToPersonID: "I2", FamilyID: "F1", Relationship: "parent"}})
	if path != "Relationship path: I1 -> I2\nHops: 1\n- I1 -> I2 via F1 (parent)" {
		t.Errorf("relationship summary = %q", path)
	}
}

func TestJoinPhrasesUsesReadableConjunctions(t *testing.T) {
	tests := []struct {
		phrases []string
		want    string
	}{
		{phrases: nil, want: ""},
		{phrases: []string{"was born"}, want: "was born"},
		{phrases: []string{"was born", "died"}, want: "was born and died"},
		{phrases: []string{"was born", "died", "worked"}, want: "was born, died, and worked"},
	}
	for _, test := range tests {
		if got := joinPhrases(test.phrases); got != test.want {
			t.Errorf("joinPhrases(%q) = %q, want %q", test.phrases, got, test.want)
		}
	}
}
