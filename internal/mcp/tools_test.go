package mcp

import (
	"testing"

	framework "github.com/mark3labs/mcp-go/mcp"
)

func TestJSONResultUsesTextContent(t *testing.T) {
	result, err := jsonResult(map[string]string{"name": "Ada"})
	if err != nil {
		t.Fatalf("jsonResult returned an error: %v", err)
	}
	if result.StructuredContent != nil {
		t.Fatalf("structured content should be omitted, got %v", result.StructuredContent)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one content item, got %d", len(result.Content))
	}
	content, ok := result.Content[0].(framework.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if content.Text != `{"name":"Ada"}` {
		t.Fatalf("unexpected content: %s", content.Text)
	}
}
