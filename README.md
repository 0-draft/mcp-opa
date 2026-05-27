# mcp-opa

[![ci](https://github.com/0-draft/mcp-opa/actions/workflows/ci.yml/badge.svg)](https://github.com/0-draft/mcp-opa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/0-draft/mcp-opa.svg)](https://pkg.go.dev/github.com/0-draft/mcp-opa)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

[MCP](https://modelcontextprotocol.io) server that exposes [OPA](https://www.openpolicyagent.org/) Rego evaluation as a tool an LLM agent can call.

Paste a Rego module and an input doc into a Claude / Cursor session. The model picks the query, `mcp-opa` runs `opa.Eval`, returns the decision set.

## Install

```bash
go install github.com/0-draft/mcp-opa@latest
```

Pre-built signed binaries are on the [releases page](https://github.com/0-draft/mcp-opa/releases).

## Quickstart

```bash
# Build and run the smoke test (no MCP client needed).
make smoke
# → ✓ smoke: allow=true
```

`make smoke` builds the binary, feeds it a synthetic MCP `initialize` →
`tools/list` → `tools/call` sequence over stdio, and asserts the policy
returned `allow=true`. It exits non-zero on protocol failure.

## Wire it to Claude Code

```bash
claude mcp add opa -- mcp-opa
```

Then in a session: paste a Rego policy, an input doc, and ask the model what
the decision should be. The model calls `evaluate_policy`, gets the answer
from OPA (not from its training data).

## Wire it to Cursor / other clients

```jsonc
{
  "mcpServers": {
    "opa": { "command": "mcp-opa" }
  }
}
```

## Tool: `evaluate_policy`

| Param        | Required | Description                                                                |
| ------------ | -------- | -------------------------------------------------------------------------- |
| `rego`       | yes      | Rego source with a `package` declaration.                                  |
| `query`      | yes      | Rego query, e.g. `data.example.allow`.                                     |
| `input_json` | no       | JSON-encoded `input` document.                                             |
| `data_json`  | no       | JSON-encoded base document for the `data` namespace.                       |

Returns the OPA `ResultSet` as JSON.

## Examples

[`examples/`](./examples/) has reference policies:

- [`rbac.rego`](./examples/rbac.rego) — role → permission mapping
- [`abac.rego`](./examples/abac.rego) — clearance level comparison
- [`k8s_admission.rego`](./examples/k8s_admission.rego) — admission control: required labels

## Layout

Flat on purpose. A single-binary MCP server with one tool does not need
`cmd/`, `internal/`, or `pkg/`. When a second tool joins, the natural split is
a sibling file (`evaluate.go`, `lint.go`, ...) — still no subpackages.

```text
.
├── main.go         # server bootstrap + tool registration
├── main_test.go
├── examples/       # reference Rego policies
├── scripts/smoke.sh
├── .goreleaser.yml
└── .github/
```

## Verify a release

Releases ship a `cosign`-signed checksum file (Sigstore keyless via GitHub OIDC) and a CycloneDX SBOM per archive.

```bash
TAG=v0.1.0
gh release download "$TAG" -R 0-draft/mcp-opa -p '*-checksums.txt*'

cosign verify-blob \
  --certificate "mcp-opa-${TAG#v}-checksums.txt.pem" \
  --signature   "mcp-opa-${TAG#v}-checksums.txt.sig" \
  --certificate-identity-regexp 'https://github.com/0-draft/mcp-opa/.github/workflows/release.yml@refs/tags/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "mcp-opa-${TAG#v}-checksums.txt"
```

## License

MIT.
