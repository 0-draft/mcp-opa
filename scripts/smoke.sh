#!/usr/bin/env bash
# Drive mcp-opa over stdio with a full MCP handshake and one tools/call.
# Useful before tagging a release, or after upgrading mcp-go.
#
# Exit codes:
#   0  decision was true (policy allowed)
#   1  decision was false (policy denied or no allow rule matched)
#   2  protocol failure (no result, no decision in payload)

set -euo pipefail

BIN="${1:-./mcp-opa}"
if [[ ! -x "$BIN" ]]; then
    echo "build first: go build ." >&2
    exit 2
fi

read -r -d '' REGO <<'EOF' || true
package smoke

default allow := false

allow if input.user == "alice"
EOF

OUT=$(printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "$(jq -nc --arg r "$REGO" '{jsonrpc:"2.0",id:2,method:"tools/call",params:{name:"evaluate_policy",arguments:{rego:$r,query:"data.smoke.allow",input_json:"{\"user\":\"alice\"}"}}}')" \
    | "$BIN")

DECISION=$(printf '%s\n' "$OUT" | jq -r 'select(.id == 2) | .result.content[0].text' | jq -r '.[0].expressions[0].value')

case "$DECISION" in
    true)  echo "✓ smoke: allow=true"; exit 0 ;;
    false) echo "✗ smoke: allow=false (expected true)"; exit 1 ;;
    *)     echo "✗ smoke: protocol failure; payload:"; printf '%s\n' "$OUT"; exit 2 ;;
esac
