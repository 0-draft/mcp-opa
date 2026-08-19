# Security Policy

## Reporting a vulnerability

Please do not open a public issue.

Use [GitHub's private vulnerability reporting](https://github.com/kanywst/mcp-opa-authz/security/advisories/new), or email <kanywst12@gmail.com>.

Include what you have: a description, steps to reproduce, the impact you think it has, and a suggested fix if you have one. Expect an acknowledgement within 48 hours.

## Supported versions

| Version | Status |
| --- | --- |
| latest release | Supported |
| older | Not supported; upgrade first |

Pre-1.0. Fixes land on `main` and in the next tag rather than being backported.

## Threat model

This server is launched as a subprocess by an MCP client and speaks JSON-RPC over stdio. Two things about that shape drive everything below:

1. **Tool arguments are attacker-influenceable.** They are composed by a language model, from context that may include a web page, a repository, an issue comment, or a document. Treat every argument as untrusted input, not as operator configuration.
2. **The process holds real credentials.** `AUTHZEN_PDP_TOKEN` is in its environment, and it runs inside whatever network boundary the developer's machine or CI runner sits behind.

### What is defended against

**Model-supplied Rego reaching the network or the host.** `evaluate_policy` compiles and evaluates policy source from the model, in-process. `http.send`, `net.lookup_ip_addr` and `opa.runtime` are removed from the OPA capability set, so a policy that calls one fails to compile rather than making an outbound request or reading the process environment. `MCP_OPA_ALLOW_NETWORK_BUILTINS=true` restores them, deliberately and with the consequences documented.

**Unbounded evaluation.** A Rego evaluation runs under `MCP_OPA_EVAL_TIMEOUT` (5s default). Without it, one expensive comprehension wedges the server for the life of the session.

**The `pdp_url` argument as an SSRF primitive.** A model-supplied endpoint must be an absolute `http`/`https` URL with a host and no userinfo. Redirects are refused rather than followed, so the `Authorization` header cannot be walked to another origin and a decision cannot be sourced from a host nobody configured.

**Resource exhaustion from a hostile PDP.** Responses are read through a byte cap (`AUTHZEN_PDP_MAX_RESPONSE_BYTES`), and every request is bounded by `AUTHZEN_PDP_TIMEOUT`.

**Untrusted text flooding the model's context.** PDP error bodies are truncated to 512 bytes. Everything an evaluated policy can put into a tool result is capped: `print()` at 200 lines of 1 KiB each, the trace at 4000 events and 200 lines of 1 KiB, the query result at 256 KiB encoded. `print()` and the trace are bounded where they are *collected*, not only where they are rendered — a cap applied at the end limits the response while the buffer grows for the whole evaluation budget.

**A non-answer being reported as a deny.** AuthZEN makes `decision` a required member, so a response omitting it means the PDP failed, not that access was denied. The distinction is preserved throughout, including for `401` and `403`, which describe *this server's* authentication to the PDP and not the subject's access.

**A panic taking down the session.** Tool handlers run under the MCP server's recovery middleware.

### What is not defended against

**The endpoint you point it at.** `pdp_url` is deliberately allowed to name a private address — a local PDP on `127.0.0.1:8181` is the common case, and blocking loopback would break the primary use. If your threat model requires an allowlist, run the server behind an egress proxy.

**Decisions your PDP gets wrong.** This forwards a question and reports the answer.

**Rego that is expensive but finishes in time.** The timeout bounds wall clock; there is no memory cap on evaluation. In particular the query result is materialised by OPA in full before its size is checked, so the 256 KiB cap governs what reaches the model, not what the process allocated getting there. `MCP_OPA_EVAL_TIMEOUT` is the only bound on that.

**A malicious MCP client.** Anything that can launch this subprocess already has the environment it runs with.

## Dependencies

CI runs `govulncheck` and `osv-scanner` on every pull request, and Dependabot opens updates for Go modules and Actions. Releases ship a CycloneDX SBOM per archive and a `cosign`-signed checksum file; see the README for verification.

Report a vulnerability in a dependency to that project, and open an issue here if this repository needs to pin or work around it.
