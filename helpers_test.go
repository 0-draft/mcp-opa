package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// resultText pulls the first text block out of a tool result. Shared by both
// tool test files.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("empty result content")
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("first content block is not text: %T", res.Content[0])
	}
	return tc.Text
}
