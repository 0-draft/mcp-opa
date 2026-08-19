package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func callPolicy(t *testing.T, cfg *config, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := evaluatePolicy(context.Background(), newRequest("evaluate_policy", args), cfg)
	if err != nil {
		t.Fatalf("evaluatePolicy returned a non-nil error, which the transport would "+
			"report as a fault instead of showing the model: %v", err)
	}
	return res
}

const adminPolicy = `package example

default allow := false

allow if input.role == "admin"`

func TestEvaluatePolicy_Allow(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":       adminPolicy,
		"query":      "data.example.allow",
		"input_json": `{"role":"admin"}`,
	})

	out := structured[evaluateResult](t, res)
	if !out.Defined {
		t.Fatal("expected the query to be defined")
	}
	if out.Value != true {
		t.Fatalf("value = %#v, want true", out.Value)
	}
	if len(out.ResultSet) != 1 {
		t.Fatalf("result set has %d entries, want 1", len(out.ResultSet))
	}
}

func TestEvaluatePolicy_Deny(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":       adminPolicy,
		"query":      "data.example.allow",
		"input_json": `{"role":"guest"}`,
	})

	out := structured[evaluateResult](t, res)
	if out.Value != false {
		t.Fatalf("value = %#v, want false", out.Value)
	}
	if !out.Defined {
		t.Fatal("a default rule makes the query defined even when it denies")
	}
}

// The distinction this test protects is the reason evaluateResult has a
// Defined field: a query with no default produces an empty result set, and
// "undefined" is not "false".
func TestEvaluatePolicy_UndefinedIsNotDeny(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if input.role == "admin"`,
		"query":      "data.example.allow",
		"input_json": `{"role":"guest"}`,
	})

	out := structured[evaluateResult](t, res)
	if out.Defined {
		t.Fatal("a rule with no default is undefined when its body fails")
	}
	if len(out.ResultSet) != 0 {
		t.Fatalf("result set should be empty, got %d entries", len(out.ResultSet))
	}
	if out.Value != nil {
		t.Fatalf("value = %#v, want null for an undefined query", out.Value)
	}
}

func TestEvaluatePolicy_DataNamespace(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if input.user in data.admins`,
		"query":      "data.example.allow",
		"input_json": `{"user":"alice"}`,
		"data_json":  `{"admins":["alice","bob"]}`,
	})

	out := structured[evaluateResult](t, res)
	if out.Value != true {
		t.Fatalf("value = %#v, want true; the data namespace was not seeded", out.Value)
	}
}

func TestEvaluatePolicy_CapturesPrintOutput(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if {
	print("checking role", input.role)
	input.role == "admin"
}`,
		"query":      "data.example.allow",
		"input_json": `{"role":"admin"}`,
	})

	out := structured[evaluateResult](t, res)
	if len(out.Printed) == 0 {
		t.Fatal("print() output was discarded")
	}
	if !strings.Contains(out.Printed[0], "checking role") {
		t.Fatalf("printed = %q, want it to contain the message", out.Printed)
	}
}

// print() is under the policy's control, and the policy came from a model. Left
// unbounded it is both a way to grow this process and a way to fill the model's
// context — the same hole the trace cap and the PDP body cap exist to close.
func TestEvaluatePolicy_BoundsPrintOutput(t *testing.T) {
	t.Run("too many lines", func(t *testing.T) {
		res := callPolicy(t, testConfig(), map[string]any{
			"rego": `package example

allow if {
	every i in numbers.range(1, 5000) {
		print(i)
	}
}`,
			"query": "data.example.allow",
		})

		out := structured[evaluateResult](t, res)
		if len(out.Printed) > maxPrintLines {
			t.Fatalf("captured %d lines, over the %d cap", len(out.Printed), maxPrintLines)
		}
		if len(out.Printed) != maxPrintLines {
			t.Fatalf("captured %d lines; the policy printed far more than the cap", len(out.Printed))
		}
		if !out.PrintedTruncated {
			t.Fatal("output was cut without saying so")
		}
	})

	t.Run("line too long", func(t *testing.T) {
		res := callPolicy(t, testConfig(), map[string]any{
			"rego": `package example

allow if print(concat("", [x | some _ in numbers.range(1, 5000); x := "AAAAAAAAAA"]))`,
			"query": "data.example.allow",
		})

		out := structured[evaluateResult](t, res)
		if len(out.Printed) != 1 {
			t.Fatalf("captured %d lines, want 1", len(out.Printed))
		}
		if len(out.Printed[0]) > maxPrintLineBytes+len("… (truncated)") {
			t.Fatalf("line is %d bytes, over the %d cap", len(out.Printed[0]), maxPrintLineBytes)
		}
		if !out.PrintedTruncated {
			t.Fatal("the line was cut without saying so")
		}
	})

	t.Run("output within the bounds is not flagged", func(t *testing.T) {
		res := callPolicy(t, testConfig(), map[string]any{
			"rego": `package example

allow if print("short")`,
			"query": "data.example.allow",
		})

		out := structured[evaluateResult](t, res)
		if out.PrintedTruncated {
			t.Fatal("output that fits must not be reported as truncated")
		}
	})
}

// The result is the one field returned on every call, and its size is set by
// the policy, not by the input — `numbers.range(1, 10000000)` is one line.
func TestEvaluatePolicy_BoundsResultSize(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

big := numbers.range(1, 500000)`,
		"query": "data.example.big",
	})

	out := structured[evaluateResult](t, res)
	if !out.ResultSetOmitted {
		t.Fatal("an oversized result set was returned in full")
	}
	if len(out.ResultSet) != 0 {
		t.Fatalf("result set was reported as omitted but carries %d entries", len(out.ResultSet))
	}
	// Whether the query had an answer is knowable even when the answer is too
	// big to hand back, so it is still reported.
	if !out.Defined {
		t.Fatal("defined must still be reported when the result is omitted")
	}
	if out.Value != nil {
		t.Fatal("a value that does not fit must not be returned either")
	}

	if encoded, err := json.Marshal(out); err != nil {
		t.Fatal(err)
	} else if len(encoded) > 4*maxResultBytes {
		t.Fatalf("result payload is %d bytes despite the cap", len(encoded))
	}
}

func TestEvaluatePolicy_ResultWithinBoundsIsReturnedWhole(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

small := numbers.range(1, 100)`,
		"query": "data.example.small",
	})

	out := structured[evaluateResult](t, res)
	if out.ResultSetOmitted {
		t.Fatal("a result that fits must not be omitted")
	}
	if len(out.ResultSet) != 1 {
		t.Fatalf("result set has %d entries, want 1", len(out.ResultSet))
	}
	if out.Value == nil {
		t.Fatal("value was dropped from a result that fits")
	}
}

func TestEncodesWithin(t *testing.T) {
	if !encodesWithin(map[string]string{"a": "b"}, 1024) {
		t.Fatal("a small value was reported as over the limit")
	}
	if encodesWithin(strings.Repeat("x", 2048), 1024) {
		t.Fatal("a large value was reported as within the limit")
	}
	// A value json cannot encode is not "within" anything.
	if encodesWithin(make(chan int), 1024) {
		t.Fatal("an unencodable value was reported as within the limit")
	}
}

func TestEvaluatePolicy_TraceOptIn(t *testing.T) {
	args := map[string]any{
		"rego":       adminPolicy,
		"query":      "data.example.allow",
		"input_json": `{"role":"guest"}`,
	}

	off := structured[evaluateResult](t, callPolicy(t, testConfig(), args))
	if len(off.Trace) != 0 {
		t.Fatalf("trace should be absent unless asked for, got %d lines", len(off.Trace))
	}

	args["trace"] = true
	on := structured[evaluateResult](t, callPolicy(t, testConfig(), args))
	if len(on.Trace) == 0 {
		t.Fatal("trace was requested but not returned")
	}
	if len(on.Trace) > maxTraceLines {
		t.Fatalf("trace is %d lines, over the %d cap", len(on.Trace), maxTraceLines)
	}
	if on.TraceTruncated {
		t.Fatal("a trace that fits must not be reported as truncated")
	}
}

// The trace is collected from a model-supplied policy, so the bound has to be
// on collection and not only on what is returned — otherwise the cap limits the
// response while the buffer grows for the whole evaluation budget and the
// renderer walks all of it.
func TestEvaluatePolicy_BoundsTraceCollection(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if {
	every i in numbers.range(1, 5000) {
		i > 0
	}
}`,
		"query": "data.example.allow",
		"trace": true,
	})

	out := structured[evaluateResult](t, res)
	if len(out.Trace) > maxTraceLines {
		t.Fatalf("trace is %d lines, over the %d cap", len(out.Trace), maxTraceLines)
	}
	if !out.TraceTruncated {
		t.Fatal("the trace was cut without saying so")
	}
	for i, line := range out.Trace {
		if len(line) > maxTraceLineLimit() {
			t.Fatalf("trace line %d is %d bytes, over the per-line cap", i, len(line))
		}
	}
}

// A trace line carries plugged local bindings, which come from input_json.
func TestEvaluatePolicy_BoundsTraceLineLength(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if input.blob != ""`,
		"query":      "data.example.allow",
		"input_json": `{"blob":"` + strings.Repeat("A", 20000) + `"}`,
		"trace":      true,
	})

	out := structured[evaluateResult](t, res)
	for i, line := range out.Trace {
		if len(line) > maxTraceLineLimit() {
			t.Fatalf("trace line %d is %d bytes, over the per-line cap", i, len(line))
		}
	}
}

// maxTraceLineLimit is the longest a capped line can be: the cap plus the
// marker appended to it.
func maxTraceLineLimit() int { return maxPrintLineBytes + len("… (truncated)") }

func TestEvaluatePolicy_RegoV0(t *testing.T) {
	// A pre-OPA-1.0 policy: no `if`, no `contains`.
	v0 := `package example

default allow = false

allow {
	input.role == "admin"
}`

	if res := callPolicy(t, testConfig(), map[string]any{
		"rego":       v0,
		"query":      "data.example.allow",
		"input_json": `{"role":"admin"}`,
	}); !res.IsError {
		t.Fatal("v0 syntax should not compile under the v1 default")
	}

	res := callPolicy(t, testConfig(), map[string]any{
		"rego":         v0,
		"query":        "data.example.allow",
		"input_json":   `{"role":"admin"}`,
		"rego_version": "v0",
	})
	out := structured[evaluateResult](t, res)
	if out.Value != true {
		t.Fatalf("value = %#v, want true under rego_version=v0", out.Value)
	}
}

func TestEvaluatePolicy_RejectsUnknownRegoVersion(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":         adminPolicy,
		"query":        "data.example.allow",
		"rego_version": "v2",
	})
	requireToolError(t, res, "rego_version")
}

// --- the sandbox ------------------------------------------------------------

// The policy source comes from a model. These three built-ins are the ones that
// can leave the process, and each has its own reason to be gone; see
// opa_capabilities.go.
func TestEvaluatePolicy_NetworkBuiltinsAreDisabled(t *testing.T) {
	policies := map[string]string{
		"http.send": `package example

allow if {
	r := http.send({"method": "GET", "url": "http://169.254.169.254/latest/meta-data/"})
	r.status_code == 200
}`,
		"net.lookup_ip_addr": `package example

allow if net.lookup_ip_addr("example.com")`,
		"opa.runtime": `package example

allow if opa.runtime().env.PATH != ""`,
	}

	for name, src := range policies {
		t.Run(name, func(t *testing.T) {
			res := callPolicy(t, testConfig(), map[string]any{
				"rego":  src,
				"query": "data.example.allow",
			})
			msg := requireToolError(t, res, name)
			if !strings.Contains(msg, envAllowNetBuilt) {
				t.Fatalf("the error should say how to re-enable it: %s", msg)
			}
		})
	}
}

func TestEvaluatePolicy_NetworkBuiltinsCanBeReEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.AllowNetworkBuiltins = true

	// Compilation is the assertion — opa.runtime() needs no network, so this
	// checks the capability set without making the test depend on one.
	res := callPolicy(t, cfg, map[string]any{
		"rego": `package example

env := opa.runtime().env`,
		"query": "data.example.env",
	})
	requireNoToolError(t, res)
}

func TestSandboxCapabilities_DropsExactlyTheNamedBuiltins(t *testing.T) {
	have := map[string]bool{}
	for _, b := range sandboxCapabilities.Builtins {
		have[b.Name] = true
	}
	for _, name := range sandboxedBuiltins {
		if have[name] {
			t.Errorf("%s is still in the sandboxed capability set", name)
		}
	}

	// Everything else survives. Time and JWT built-ins in particular are load
	// bearing in real authorization policies, so an over-broad filter — every
	// nondeterministic built-in, say — would be a regression.
	for _, name := range []string{"time.now_ns", "io.jwt.decode_verify", "uuid.rfc4122", "rand.intn"} {
		if !have[name] {
			t.Errorf("%s was removed; the sandbox is meant to drop only network and host access", name)
		}
	}
	if len(sandboxCapabilities.Builtins) != len(baseCapabilities.Builtins)-len(sandboxedBuiltins) {
		t.Errorf("dropped %d built-ins, want %d",
			len(baseCapabilities.Builtins)-len(sandboxCapabilities.Builtins), len(sandboxedBuiltins))
	}
	if baseCapabilities.AllowNet != nil {
		t.Error("the base capability set was mutated; it must stay shared and untouched")
	}
	if sandboxCapabilities.AllowNet == nil || len(sandboxCapabilities.AllowNet) != 0 {
		t.Error("sandbox AllowNet must be empty-and-non-nil, which OPA reads as \"no host\"")
	}
}

// --- bounds and failure modes -----------------------------------------------

func TestEvaluatePolicy_TimesOut(t *testing.T) {
	cfg := shortTimeoutConfig(50 * time.Millisecond)

	// A cross product large enough to outlast the deadline but small enough to
	// not matter if the deadline somehow fails to fire.
	res := callPolicy(t, cfg, map[string]any{
		"rego": `package example

n := numbers.range(1, 700)

allow contains [a, b, c] if {
	some a in n
	some b in n
	some c in n
	a + b + c == 42
}`,
		"query": "data.example.allow",
	})
	requireToolError(t, res, envRegoTimeout)
}

// The caller's own cancellation must not be reported as the evaluation
// exceeding its budget — the two have different fixes.
func TestEvaluatePolicy_CallerCancellationIsNotATimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := evaluatePolicy(ctx, newRequest("evaluate_policy", map[string]any{
		"rego":  adminPolicy,
		"query": "data.example.allow",
	}), testConfig())
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError && strings.Contains(resultText(t, res), envRegoTimeout) {
		t.Fatalf("a cancelled caller was reported as an evaluation timeout: %s", resultText(t, res))
	}
}

func TestEvaluatePolicy_RejectsOversizedArguments(t *testing.T) {
	cfg := testConfig()
	cfg.MaxArgBytes = 64

	res := callPolicy(t, cfg, map[string]any{
		"rego":  "package example\n\n" + strings.Repeat("# padding\n", 100),
		"query": "data.example.allow",
	})
	requireToolError(t, res, "over the 64 byte limit")
}

func TestEvaluatePolicy_BadRego(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":  "this is not valid rego",
		"query": "data.example.allow",
	})
	requireToolError(t, res, "rego compile error")
}

func TestEvaluatePolicy_BadInputJSON(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":       adminPolicy,
		"query":      "data.example.allow",
		"input_json": "{not json}",
	})
	requireToolError(t, res, "input_json")
}

func TestEvaluatePolicy_RejectsNonObjectData(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":      adminPolicy,
		"query":     "data.example.allow",
		"data_json": `["not","an","object"]`,
	})
	requireToolError(t, res, "data_json")
}

func TestEvaluatePolicy_MissingRequiredArgs(t *testing.T) {
	for _, missing := range []string{"rego", "query"} {
		t.Run(missing, func(t *testing.T) {
			args := map[string]any{"rego": adminPolicy, "query": "data.example.allow"}
			delete(args, missing)
			requireToolError(t, callPolicy(t, testConfig(), args), missing)
		})
	}
}

// A built-in that fails at runtime — here, a type error — must surface as an
// error and not quietly become an undefined result the model reads as a deny.
func TestEvaluatePolicy_StrictBuiltinErrors(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

allow if {
	x := split(input.value, ",")
	count(x) > 0
}`,
		"query":      "data.example.allow",
		"input_json": `{"value": 42}`,
	})
	requireToolError(t, res, "eval")
}

// The text fallback must stay parseable on its own: a client on an older MCP
// revision never sees structuredContent.
func TestEvaluatePolicy_TextFallbackIsSelfContained(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego":       adminPolicy,
		"query":      "data.example.allow",
		"input_json": `{"role":"admin"}`,
	})

	var out evaluateResult
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("text content is not decodable JSON: %v", err)
	}
	if out.Value != true {
		t.Fatalf("text fallback value = %#v, want true", out.Value)
	}
}

func TestSingleValue(t *testing.T) {
	res := callPolicy(t, testConfig(), map[string]any{
		"rego": `package example

pair contains x if some x in [1, 2]`,
		"query": "data.example.pair[_]",
	})
	out := structured[evaluateResult](t, res)
	if len(out.ResultSet) != 2 {
		t.Fatalf("result set has %d entries, want 2", len(out.ResultSet))
	}
	if out.Value != nil {
		t.Fatalf("value = %#v; it is only meaningful for a single-result query", out.Value)
	}
}
