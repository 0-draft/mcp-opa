# Contributing

Thanks for looking. This is a small, focused server; the bar for a change is that it makes an authorization answer more trustworthy, not that it adds a feature.

## Setup

You need Go (the version in `go.mod`), `make`, and — for `make smoke` — `jq` and `python3`.

```bash
git clone https://github.com/kanywst/mcp-opa-authz.git
cd mcp-opa-authz
make test
make smoke
```

## The loop

```bash
make fmt          # gofmt -s
make vet          # go vet
make lint         # golangci-lint, config in .golangci.yml
make test         # go test -race
make smoke        # build + end-to-end MCP stdio session against a fake PDP
```

Run one test:

```bash
go test -race -run TestEvaluate_MissingDecisionIsAnErrorNotADeny -v ./...
```

CI runs all of the above plus `govulncheck`, `osv-scanner` and `actionlint`. Get `make lint && make test && make smoke` green locally and CI will almost certainly agree.

## Good first contributions

- **Another example policy** in `examples/`. Each one carries a header comment with its query and a sample input.
- **A PDP this has not been tried against.** If `authzen_evaluate` works — or doesn't — against a PDP not mentioned in the README, that is worth an issue either way.
- **A gap against the [AuthZEN 1.0 final text](https://openid.net/specs/authorization-api-1_0.html).** Member names and request shapes are normative; a mismatch is a real bug even when both sides still parse.
- **The Search APIs.** Subject, resource and action search are specified and not implemented here.

## Things worth knowing before you change the code

These are the invariants the tests exist to protect. Breaking one is not a refactor.

**A failure is never a deny.** `evaluationResponse.Decision` is a `*bool` on purpose: AuthZEN makes the member required, so a response without it means the PDP did not answer. Decoded into a plain `bool` it would arrive as `false`, and a broken PDP would be indistinguishable from a strict one. Every path that produces a decision has to keep these separate — including HTTP status handling, where `401` means *this server* failed to authenticate.

**Tool handlers return `error == nil`.** Every user-facing failure goes back as `mcp.NewToolResultError` with a nil Go error. A non-nil error surfaces as a transport fault and the model never sees the message. The tests assert `res.IsError`, never a non-nil error.

**Batch decisions carry an explicit index.** `deny_on_first_deny` and `permit_on_first_permit` legitimately return fewer decisions than there were entries, and the result reports `truncated`. Zipping the two arrays positionally would silently attach a decision to the wrong resource.

**The Rego sandbox is load-bearing.** `evaluate_policy` compiles source that came from a model and runs it in this process. `opa_capabilities.go` removes `http.send`, `net.lookup_ip_addr` and `opa.runtime`. Widening that set, bypassing `cfg.capabilities()`, or dropping the evaluation deadline is a security change and needs to be argued for as one.

**Everything echoed to the model is bounded.** PDP response bodies, error snippets, traces. A new path that quotes untrusted text needs a cap.

**Entities go to the PDP byte for byte.** Subject, action, resource and context stay as `json.RawMessage`. A decode/re-encode round trip reorders members and turns integers into floats, and a policy keyed on either would then answer a different question.

## Structure

Flat `package main`. One tool per `tool_*.go`, registered from `main.go`; shared machinery in `config.go`, `args.go`, `authzen.go`, `opa_capabilities.go`. Adding a tool means a new sibling file and one more `register…` call — not a package hierarchy.

## Pull requests

- One logical change per PR. Keep refactors separate from behaviour changes.
- Add a test that fails without your fix. For anything touching a decision path, add the case where the PDP or the policy misbehaves, not only the happy path.
- Commit messages in English, conventional-commits style: `feat(authzen): …`, `fix: …`, `docs: …`, `chore(deps): …`.
- Update `CHANGELOG.md` under `## Unreleased` if the change is user-visible.

A `Claude review` job comments on pull requests from this repository. It reviews; it does not gate. A red run means the review did not happen, not that the code is wrong.

## Reporting security issues

Do not open a public issue. See [SECURITY.md](./SECURITY.md).
