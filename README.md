# mcp-opa

[![ci](https://github.com/0-draft/mcp-opa/actions/workflows/ci.yml/badge.svg)](https://github.com/0-draft/mcp-opa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/0-draft/mcp-opa.svg)](https://pkg.go.dev/github.com/0-draft/mcp-opa)

A [Model Context Protocol](https://modelcontextprotocol.io) server that lets an LLM agent run [Open Policy Agent](https://www.openpolicyagent.org/) Rego evaluations as a tool.

Useful for "let Claude reason about a policy" workflows: paste a Rego module + an input doc into a chat and ask the model to walk through what happens. The model writes the query; `mcp-opa` runs OPA and returns the decision set.

## Tool

### `evaluate_policy`

Evaluate a Rego module against an input document and optional `data` namespace.

| Parameter    | Required | Description                                                                              |
| ------------ | -------- | ---------------------------------------------------------------------------------------- |
| `rego`       | yes      | Rego source code. Must include a `package` declaration.                                  |
| `query`      | yes      | Query to evaluate, e.g. `data.example.allow` or `data.example.violations[_]`.            |
| `input_json` | no       | JSON-encoded input document (becomes the `input` variable inside Rego).                  |
| `data_json`  | no       | JSON-encoded base document seeding the `data` namespace via OPA's in-memory store.       |

The tool returns the OPA `ResultSet` as JSON.

## Install

```bash
go install github.com/0-draft/mcp-opa@latest
```

Or grab a signed binary from the [releases page](https://github.com/0-draft/mcp-opa/releases).

## Use with Claude Code

```bash
claude mcp add opa -- mcp-opa
```

Then in a Claude Code session:

> Evaluate this RBAC policy against the request — does Alice get to delete the document?
>
> ```rego
> package rbac
> default allow := false
> allow if input.user == "alice" and input.action == "read"
> ```

Claude calls `evaluate_policy` with the right query, returns the decision.

## Use with Cursor / other MCP clients

Add to your client's MCP server config:

```json
{
  "mcpServers": {
    "opa": {
      "command": "mcp-opa"
    }
  }
}
```

## Verifying a release

Each release ships a `cosign`-signed checksum file (keyless, Sigstore via GitHub OIDC) and a CycloneDX SBOM. To verify before installing:

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
