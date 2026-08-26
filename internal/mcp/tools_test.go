package mcp

import (
	"testing"
)

func TestJSONResultUsesStructuredContent(t *testing.T) {
	result, err := jsonResult(map[string]string{"name": "Ada"})
	if err != nil {
		t.Fatalf("jsonResult returned an error: %v", err)
	}
	if result.StructuredContent == nil {
		t.Fatal("structured content should be present")
	}
	if len(result.Content) != 0 {
		t.Fatalf("expected no content items, got %d", len(result.Content))
	}
}
