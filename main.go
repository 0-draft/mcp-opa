// mcp-opa is a Model Context Protocol (MCP) server that exposes
// Open Policy Agent (OPA) Rego evaluation as a callable tool. Designed to be
// launched as a subprocess by an MCP client (Claude Code, Cursor, etc.) over
// stdio.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("mcp-opa %s\n", version)
			return
		case "--help", "-h", "help":
			fmt.Println(`mcp-opa — MCP server exposing OPA/Rego policy evaluation as a tool.

Usage:
  mcp-opa             Run as an MCP stdio server (default).
  mcp-opa --version   Print version.

Tools exposed:
  evaluate_policy     Evaluate a Rego module against an input document.

Configure with an MCP client (Claude Code example):
  claude mcp add opa -- mcp-opa`)
			return
		}
	}

	s := server.NewMCPServer(
		"mcp-opa",
		version,
		server.WithToolCapabilities(false),
	)

	s.AddTool(
		mcp.NewTool("evaluate_policy",
			mcp.WithDescription(
				"Evaluate a Rego policy module against an input document and "+
					"optional data namespace. Returns the resulting decision "+
					"set as JSON."),
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

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp-opa: %v", err)
	}
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
