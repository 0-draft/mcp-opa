package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func callTool(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "evaluate_policy"
	req.Params.Arguments = args
	res, err := evaluatePolicy(context.Background(), req)
	if err != nil {
		t.Fatalf("evaluatePolicy returned non-nil err: %v", err)
	}
	return res
}

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

func TestEvaluatePolicy_Allow(t *testing.T) {
	res := callTool(t, map[string]any{
		"rego": `package example

default allow := false

allow if {
	input.role == "admin"
}`,
		"query":      "data.example.allow",
		"input_json": `{"role": "admin"}`,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, res))
	}

	var rs []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, res)), &rs); err != nil {
		t.Fatalf("result is not a JSON array: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("empty result set; expected one binding with allow=true")
	}

	exprs, ok := rs[0]["expressions"].([]any)
	if !ok || len(exprs) == 0 {
		t.Fatalf("missing expressions in result: %#v", rs[0])
	}
	first := exprs[0].(map[string]any)
	if first["value"] != true {
		t.Fatalf("expected allow=true, got: %#v", first["value"])
	}
}

func TestEvaluatePolicy_Deny(t *testing.T) {
	res := callTool(t, map[string]any{
		"rego": `package example
default allow := false`,
		"query":      "data.example.allow",
		"input_json": `{}`,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), `"value": false`) {
		t.Fatalf("expected allow=false in output: %s", resultText(t, res))
	}
}

func TestEvaluatePolicy_BadRego(t *testing.T) {
	res := callTool(t, map[string]any{
		"rego":  `this is not valid rego`,
		"query": "data.example.allow",
	})
	if !res.IsError {
		t.Fatal("expected IsError=true for invalid rego")
	}
}

func TestEvaluatePolicy_BadInputJSON(t *testing.T) {
	res := callTool(t, map[string]any{
		"rego": `package example
allow := true`,
		"query":      "data.example.allow",
		"input_json": `{not json}`,
	})
	if !res.IsError {
		t.Fatal("expected IsError=true for malformed input_json")
	}
}
