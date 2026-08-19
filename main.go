// mcp-opa-authz is a Model Context Protocol (MCP) server that gives an LLM
// agent two ways to get an authorization answer from real policy code instead
// of from its own guess:
//
//	evaluate_policy    Evaluate a Rego module locally, in-process, via OPA.
//	authzen_evaluate   Ask a remote OpenID AuthZEN 1.0 PDP for a decision.
//
// The two cover the same question at different layers. `evaluate_policy` is for
// authoring and debugging a policy you have the source of; `authzen_evaluate`
// is for asking the PDP that actually governs a system at runtime.
//
// Designed to be launched as a subprocess by an MCP client (Claude Code,
// Cursor, etc.) over stdio.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("mcp-opa-authz %s\n", version)
			return
		case "--help", "-h", "help":
			fmt.Println(`mcp-opa-authz — MCP server exposing OPA/Rego evaluation and an OpenID AuthZEN 1.0 PDP.

Usage:
  mcp-opa-authz             Run as an MCP stdio server (default).
  mcp-opa-authz --version   Print version.

Tools exposed:
  evaluate_policy     Evaluate a Rego module against an input document,
                      in-process. No external service required.
  authzen_evaluate    POST a subject/resource/action/context bundle to an
                      AuthZEN 1.0 PDP and return its decision.

Configuration (authzen_evaluate only):
  AUTHZEN_PDP_URL     Default PDP evaluation endpoint, e.g.
                      http://localhost:8181/access/v1/evaluation
  AUTHZEN_PDP_TOKEN   Optional Authorization header value. A bare token is
                      sent as "Bearer <token>".

Configure with an MCP client (Claude Code example):
  claude mcp add opa-authz -- mcp-opa-authz`)
			return
		}
	}

	s := server.NewMCPServer(
		"mcp-opa-authz",
		version,
		server.WithToolCapabilities(false),
	)

	registerOPATool(s)
	registerAuthZENTool(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp-opa-authz: %v", err)
	}
}
