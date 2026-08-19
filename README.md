# mcp-opa-authz

[![ci](https://github.com/kanywst/mcp-opa-authz/actions/workflows/ci.yml/badge.svg)](https://github.com/kanywst/mcp-opa-authz/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kanywst/mcp-opa-authz.svg)](https://pkg.go.dev/github.com/kanywst/mcp-opa-authz)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

[MCP](https://modelcontextprotocol.io) server that lets an LLM agent get an authorization answer from real policy code instead of from its own guess.

Two tools, same question at two layers:

| Tool | Answers | Needs |
| --- | --- | --- |
| `evaluate_policy` | "What does this Rego say?" Evaluated in-process by [OPA](https://www.openpolicyagent.org/). | Nothing external |
| `authzen_evaluate` | "What does the PDP that actually governs this system say?" Sent to an [OpenID AuthZEN 1.0](https://openid.net/specs/authorization-api-1_0.html) PDP. | A reachable PDP |

Use `evaluate_policy` while authoring or debugging a policy you have the source of. Use `authzen_evaluate` when the decision has to come from the PDP in production, not from a policy pasted into the chat.

This repo is the merge of the former `0-draft/mcp-opa` and `0-draft/mcp-authzen`. Both histories are preserved here.

## Install

```bash
go install github.com/kanywst/mcp-opa-authz@latest
```

Pre-built signed binaries are on the [releases page](https://github.com/kanywst/mcp-opa-authz/releases).

## Quickstart

```bash
# Build and run the smoke test (no MCP client, no real PDP needed).
make smoke
# → ✓ smoke: evaluate_policy allow=true
# → ✓ smoke: authzen_evaluate decision=true forwarded from fake PDP
```

`make smoke` builds the binary, stands up a fake AuthZEN PDP, feeds the server a synthetic MCP `initialize` → `tools/call` → `tools/call` sequence over stdio, and asserts both tools answered. It exits non-zero on protocol failure.

## Wire it to Claude Code

```bash
claude mcp add opa-authz -- mcp-opa-authz
```

For `authzen_evaluate`, point it at a PDP:

```bash
claude mcp add opa-authz --env AUTHZEN_PDP_URL=http://localhost:8181/access/v1/evaluation -- mcp-opa-authz
```

A local [opa-authzen-plugin](https://github.com/kanywst/opa-authzen-plugin) on `:8181` works as that PDP.

## Wire it to Cursor / other clients

```jsonc
{
  "mcpServers": {
    "opa-authz": {
      "command": "mcp-opa-authz",
      "env": { "AUTHZEN_PDP_URL": "http://localhost:8181/access/v1/evaluation" }
    }
  }
}
```

## Tool: `evaluate_policy`

| Param | Required | Description |
| --- | --- | --- |
| `rego` | yes | Rego source with a `package` declaration. |
| `query` | yes | Rego query, e.g. `data.example.allow`. |
| `input_json` | no | JSON-encoded `input` document. |
| `data_json` | no | JSON-encoded base document for the `data` namespace. |

Returns the OPA `ResultSet` as JSON.

## Tool: `authzen_evaluate`

| Param | Required | Description |
| --- | --- | --- |
| `subject` | yes | JSON object describing the principal, e.g. `{"type":"user","id":"alice"}`. |
| `resource` | yes | JSON object describing the target, e.g. `{"type":"document","id":"doc-1"}`. |
| `action` | yes | JSON object describing the action, e.g. `{"name":"read"}`. |
| `context` | no | JSON object with runtime context (IP, time, MFA strength). |
| `pdp_url` | no | Override `AUTHZEN_PDP_URL` for this call. |

Returns the PDP's AuthZEN response (`decision` plus optional `context`).

Conforms to AuthZEN 1.0 §6 (single-evaluation request/response) and §5.5 (decision entity). Batch evaluation (§7) is not yet exposed as a tool. File an issue if you need it.

### Environment

| Variable | Description |
| --- | --- |
| `AUTHZEN_PDP_URL` | Default PDP evaluation endpoint. |
| `AUTHZEN_PDP_TOKEN` | Optional `Authorization` header value. A bare token is sent as `Bearer <token>`. |

A model-supplied `pdp_url` is constrained to absolute `http`/`https` URLs with a host, so the tool can't be steered into non-HTTP schemes. Responses are capped at 1 MiB.

## Examples

[`examples/`](./examples/) has reference Rego policies for `evaluate_policy`:

- [`rbac.rego`](./examples/rbac.rego) — role to permission mapping
- [`abac.rego`](./examples/abac.rego) — clearance level comparison
- [`k8s_admission.rego`](./examples/k8s_admission.rego) — admission control: required labels

## Layout

Flat on purpose. A single-binary MCP server does not need `cmd/`, `internal/`, or `pkg/`. Each tool is a sibling file.

```text
.
├── main.go             # server bootstrap, help, tool registration
├── tool_opa.go         # evaluate_policy
├── tool_authzen.go     # authzen_evaluate
├── helpers_test.go
├── tool_opa_test.go
├── tool_authzen_test.go
├── examples/           # reference Rego policies
├── scripts/smoke.sh
├── .goreleaser.yml
└── .github/
```

## Verify a release

Releases ship a `cosign`-signed checksum file (Sigstore keyless via GitHub OIDC) and a CycloneDX SBOM per archive.

```bash
TAG=v0.1.0
gh release download "$TAG" -R kanywst/mcp-opa-authz -p '*-checksums.txt*'

cosign verify-blob \
  --certificate "mcp-opa-authz-${TAG#v}-checksums.txt.pem" \
  --signature   "mcp-opa-authz-${TAG#v}-checksums.txt.sig" \
  --certificate-identity-regexp 'https://github.com/kanywst/mcp-opa-authz/.github/workflows/release.yml@refs/tags/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "mcp-opa-authz-${TAG#v}-checksums.txt"
```

## License

MIT.
