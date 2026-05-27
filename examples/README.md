# Examples

Reference Rego policies you can paste into `evaluate_policy` to see how `mcp-opa` behaves.

| File                                       | Pattern                                       |
| ------------------------------------------ | --------------------------------------------- |
| [`rbac.rego`](./rbac.rego)                 | Role-based: roles → permissions               |
| [`abac.rego`](./abac.rego)                 | Attribute-based: subject + resource matching  |
| [`k8s_admission.rego`](./k8s_admission.rego) | Kubernetes admission control (require labels) |

## Running an example end-to-end

```bash
# 1. Start mcp-opa from any MCP client config (Claude Code shown):
claude mcp add opa -- mcp-opa

# 2. In a session, ask:
#    "Evaluate examples/rbac.rego for alice trying to delete document doc-1.
#     What's the query?"
#
#    Claude will read the rego, pick `data.rbac.allow`, send input
#    {"user": "alice", "action": "delete", "resource": "doc-1"},
#    and read back the decision.
```

You can also drive it manually over stdio (advanced; MCP is JSON-RPC over stdin/stdout):

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | mcp-opa
```
