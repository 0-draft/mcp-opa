package main

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func registerOPATool(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("evaluate_policy",
			mcp.WithDescription(
				"Evaluate a Rego policy module against an input document and "+
					"optional data namespace. Runs in-process via OPA, no "+
					"external service. Returns the resulting decision set as "+
					"JSON."),
			mcp.WithString("rego",
				mcp.Required(),
				mcp.Description("Rego source code defining the policy. "+
					"Must include a package declaration."),
			),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Rego query to evaluate, e.g. "+
					"'data.example.allow' or 'data.example.violations[_]'."),
			),
			mcp.WithString("input_json",
				mcp.Description("JSON-encoded input document (the "+
					"`input` variable inside Rego)."),
			),
			mcp.WithString("data_json",
				mcp.Description("JSON-encoded base document seeding the "+
					"`data` namespace (in-memory store)."),
			),
		),
		evaluatePolicy,
	)
}

func evaluatePolicy(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	regoSrc, err := req.RequireString("rego")
	if err != nil {
		return mcp.NewToolResultError("missing required arg `rego`: " + err.Error()), nil
	}
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("missing required arg `query`: " + err.Error()), nil
	}

	var input any
	if s := req.GetString("input_json", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &input); err != nil {
			return mcp.NewToolResultError("input_json is not valid JSON: " + err.Error()), nil
		}
	}

	options := []func(*rego.Rego){
		rego.Query(query),
		rego.Module("policy.rego", regoSrc),
	}

	if s := req.GetString("data_json", ""); s != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(s), &data); err != nil {
			return mcp.NewToolResultError("data_json is not valid JSON object: " + err.Error()), nil
		}
		options = append(options, rego.Store(inmem.NewFromObject(data)))
	}

	prepared, err := rego.New(options...).PrepareForEval(ctx)
	if err != nil {
		return mcp.NewToolResultError("rego prepare error: " + err.Error()), nil
	}
	rs, err := prepared.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return mcp.NewToolResultError("rego eval error: " + err.Error()), nil
	}

	out, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to marshal result: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(out)), nil
}
