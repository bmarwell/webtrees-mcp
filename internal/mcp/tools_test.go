package mcp

import (
	"reflect"
	"testing"

	framework "github.com/mark3labs/mcp-go/mcp"
	"webtrees-mcp/internal/model"
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

func TestPersonSummaryAddsOptionalPhrases(t *testing.T) {
	tests := []struct {
		name   string
		person model.PersonDTO
		want   string
	}{
		{
			name: "complete",
			person: model.PersonDTO{
				ID: "I44", Name: model.NameDTO{Given: "Ada", Surname: "Mayer"},
				BirthDate: "1982", DeathDate: "2026", Occupation: "Carpenter",
				Relatives: []model.RelativeLink{{PersonID: "I45", Relationship: "spouse"}},
			},
			want: `Person "Ada Mayer" (I44) was born on 1982 and died on 2026. They worked as Carpenter. Associated people: I45 (spouse).`,
		},
		{
			name:   "missing optional fields",
			person: model.PersonDTO{ID: "I44", Name: model.NameDTO{Given: "Ada"}},
			want:   `Person "Ada" (I44) has no recorded birth date.`,
		},
		{
			name: "death without birth",
			person: model.PersonDTO{
				ID: "I45", Name: model.NameDTO{Given: "John", Surname: "Doe"}, DeathDate: "2026",
			},
			want: `Person "John Doe" (I45) has no recorded birth date and died on 2026.`,
		},
		{
			name: "relative without relationship",
			person: model.PersonDTO{
				ID: "I46", Relatives: []model.RelativeLink{{PersonID: "I44"}},
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
	if got := familySummary(model.FamilyDTO{ID: "F1"}); got != "Family F1." {
		t.Errorf("familySummary() = %q", got)
	}
	if got := familySummary(model.FamilyDTO{
		ID:       "F1",
		Parents:  []model.RelativeLink{{PersonID: "I1"}, {PersonID: "I2"}},
		Children: []model.RelativeLink{{PersonID: "I3"}, {PersonID: "I4"}},
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
