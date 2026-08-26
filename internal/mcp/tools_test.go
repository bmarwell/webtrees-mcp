package mcp

import (
	"testing"
)

func TestStructuredResultUsesStructuredContent(t *testing.T) {
	result, err := structuredResult(map[string]string{"name": "Ada"}, "Found Ada.")
	if err != nil {
		t.Fatalf("jsonResult returned an error: %v", err)
	}
	if result.StructuredContent == nil {
		t.Fatal("structured content should be present")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one content item, got %d", len(result.Content))
	}
}
