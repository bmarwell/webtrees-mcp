package mcp

import (
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
	for _, name := range []string{"get_person", "search_persons", "get_family", "list_tree_ids", "list_recently_born", "list_recently_deceased"} {
		tool := mcpServer.GetTool(name)
		if tool == nil {
			t.Fatalf("tool %q was not registered", name)
		}
		if !strings.Contains(tool.Tool.Description, "Use when") {
			t.Errorf("tool %q lacks selection guidance: %q", name, tool.Tool.Description)
		}
		if tool.Tool.OutputSchema.Type != "object" {
			t.Errorf("tool %q output schema type = %q, want object", name, tool.Tool.OutputSchema.Type)
		}
	}
	searchTool := mcpServer.GetTool("search_persons")
	if _, ok := searchTool.Tool.InputSchema.Properties["include_indirect"]; !ok {
		t.Error("search_persons lacks include_indirect input metadata")
	}
	for _, name := range []string{"list_tree_ids", "list_recently_born", "list_recently_deceased"} {
		tool := mcpServer.GetTool(name)
		for _, argument := range []string{"limit", "offset"} {
			if _, ok := tool.Tool.InputSchema.Properties[argument]; !ok {
				t.Errorf("%s lacks %s input metadata", name, argument)
			}
		}
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
	trees := treesResult([]domain.Tree{{ID: "tree-1"}})
	if len(trees.Trees) != 1 || trees.Trees[0].ID != "tree-1" {
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

func TestSearchPersonResultIncludesMatchMetadata(t *testing.T) {
	result := searchPersonResult(domain.PersonSearchResult{
		Person: domain.Person{ID: "I44"},
		Match:  domain.SearchMatch{DirectHit: false, Fields: []string{"gedcom_record"}},
	})
	if result.Match == nil || result.Match.DirectHit || len(result.Match.Fields) != 1 || result.Match.Fields[0] != "gedcom_record" {
		t.Fatalf("unexpected search match metadata: %+v", result.Match)
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
			want: `Person "Ada Mayer" (I44) was born on 1982 and died on 2026. They worked as Carpenter. Associated people: I45 (spouse).`,
		},
		{
			name:   "missing optional fields",
			person: domain.Person{ID: "I44", Name: domain.Name{Given: "Ada"}},
			want:   `Person "Ada" (I44) has no recorded birth date.`,
		},
		{
			name: "death without birth",
			person: domain.Person{
				ID: "I45", Name: domain.Name{Given: "John", Surname: "Doe"}, DeathDate: "2026",
			},
			want: `Person "John Doe" (I45) has no recorded birth date and died on 2026.`,
		},
		{
			name: "relative without relationship",
			person: domain.Person{
				ID: "I46", Relatives: []domain.Relative{{PersonID: "I44"}},
			},
			want: `Person "Unknown person" (I46) has no recorded birth date. Associated people: I44.`,
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

func TestFamilyAndTreeSummariesHandleOptionalData(t *testing.T) {
	if got := familySummary(domain.Family{ID: "F1"}); got != "Family F1." {
		t.Errorf("familySummary() = %q", got)
	}
	if got := familySummary(domain.Family{
		ID:       "F1",
		Parents:  []domain.Relative{{PersonID: "I1"}, {PersonID: "I2"}},
		Children: []domain.Relative{{PersonID: "I3"}, {PersonID: "I4"}},
	}); got != "Family F1 has parents I1 and I2; children: I3, I4." {
		t.Errorf("familySummary() = %q", got)
	}
	if got := treesSummary(nil); got != "No trees found." {
		t.Errorf("treesSummary(nil) = %q", got)
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
