package main

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// The Rego handed to evaluate_policy is written by a language model, from text
// that may itself have come from a web page, a repository, or a chat message.
// It is executed in this process. That is fine for the arithmetic and set logic
// a policy is made of, and not fine for the handful of built-ins that reach
// past the evaluator:
//
//	http.send           issues arbitrary HTTP requests from wherever this
//	                    server runs, which for a stdio MCP server is the
//	                    developer's laptop or a CI runner — inside whatever
//	                    network boundary those sit behind. It is the entire
//	                    SSRF surface of the tool, and it contradicts the tool's
//	                    own description ("runs in-process, no external
//	                    service").
//	net.lookup_ip_addr  resolves attacker-chosen names, which is enough to
//	                    exfiltrate the contents of `input` one DNS label at a
//	                    time even when no port is reachable.
//	opa.runtime         returns the runtime configuration, which includes the
//	                    process environment — every credential in it.
//
// None of the three appears in a policy that is being authored or debugged,
// which is what this tool is for. They are removed from the capability set
// rather than blocked at call time so that a policy using one fails to compile,
// with a message naming the built-in, instead of failing somewhere inside
// evaluation.
var sandboxedBuiltins = []string{
	ast.HTTPSend.Name,
	ast.NetLookupIPAddr.Name,
	ast.OPARuntime.Name,
}

// baseCapabilities is the full, unmodified capability set for the OPA version
// compiled in. Computed once: CapabilitiesForThisVersion walks every built-in
// on every call.
var baseCapabilities = ast.CapabilitiesForThisVersion()

// sandboxCapabilities is baseCapabilities minus sandboxedBuiltins.
var sandboxCapabilities = withoutBuiltins(baseCapabilities, sandboxedBuiltins)

// capabilities returns the capability set a single evaluation should run under.
func (c *config) capabilities() *ast.Capabilities {
	if c.AllowNetworkBuiltins {
		return baseCapabilities
	}
	return sandboxCapabilities
}

// withoutBuiltins copies caps with the named built-ins removed. AllowNet is
// pinned to an empty non-nil slice at the same time: it governs whether the
// type checker may fetch remote JSON Schemas during compilation, and its
// documented meaning is "any host" when omitted and "no host" when empty.
func withoutBuiltins(caps *ast.Capabilities, names []string) *ast.Capabilities {
	out := *caps
	out.Builtins = make([]*ast.Builtin, 0, len(caps.Builtins))
	for _, b := range caps.Builtins {
		if slices.Contains(names, b.Name) {
			continue
		}
		out.Builtins = append(out.Builtins, b)
	}
	out.AllowNet = []string{}
	return &out
}
