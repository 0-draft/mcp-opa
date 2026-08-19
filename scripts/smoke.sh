#!/usr/bin/env bash
# End-to-end smoke test for mcp-opa-authz. Drives one MCP stdio session that
# exercises both tools:
#
#   1. Starts a fake AuthZEN PDP (Python) on a free port that always returns
#      {"decision": true}.
#   2. Runs an MCP initialize → tools/call(evaluate_policy) →
#      tools/call(authzen_evaluate) sequence over stdio.
#   3. Asserts the Rego policy allowed, and that the PDP decision came back.
#
# Useful before tagging a release, or after upgrading mcp-go / OPA.
#
# Exit codes:
#   0  both tools answered correctly
#   1  a tool returned the wrong decision
#   2  protocol / setup failure

set -euo pipefail

BIN="${1:-./mcp-opa-authz}"
if [[ ! -x "$BIN" ]]; then
    echo "build first: go build ." >&2
    exit 2
fi

PORT=18181
PDP_LOG=$(mktemp)
# shellcheck disable=SC2329 # registered as EXIT trap below
cleanup() {
    if [[ -n "${PDP_PID:-}" ]] && kill -0 "$PDP_PID" 2>/dev/null; then
        kill "$PDP_PID" 2>/dev/null || true
    fi
    rm -f "$PDP_LOG"
}
trap cleanup EXIT

python3 - "$PORT" >"$PDP_LOG" 2>&1 <<'PY' &
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class FakePDP(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        _ = self.rfile.read(length)
        body = json.dumps({"decision": True}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), FakePDP).serve_forever()
PY
PDP_PID=$!
disown "$PDP_PID" 2>/dev/null || true

# Wait for the fake PDP to be listening (pure-bash TCP probe)
for _ in {1..30}; do
    (echo >/dev/tcp/127.0.0.1/$PORT) 2>/dev/null && break
    sleep 0.1
done

read -r -d '' REGO <<'EOF' || true
package smoke

default allow := false

allow if input.user == "alice"
EOF

OUT=$(printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "$(jq -nc --arg r "$REGO" '{jsonrpc:"2.0",id:2,method:"tools/call",params:{name:"evaluate_policy",arguments:{rego:$r,query:"data.smoke.allow",input_json:"{\"user\":\"alice\"}"}}}')" \
    '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"authzen_evaluate","arguments":{"subject":"{\"type\":\"user\",\"id\":\"alice\"}","resource":"{\"type\":\"doc\",\"id\":\"d1\"}","action":"{\"name\":\"read\"}"}}}' \
    | AUTHZEN_PDP_URL="http://127.0.0.1:${PORT}/access/v1/evaluation" "$BIN")

ALLOW=$(printf '%s\n' "$OUT" | jq -r 'select(.id == 2) | .result.content[0].text' | jq -r '.[0].expressions[0].value')
DECISION=$(printf '%s\n' "$OUT" | jq -r 'select(.id == 3) | .result.content[0].text' | jq -r '.decision // empty')

case "$ALLOW" in
    true)  echo "✓ smoke: evaluate_policy allow=true" ;;
    false) echo "✗ smoke: evaluate_policy allow=false (expected true)"; exit 1 ;;
    *)     echo "✗ smoke: evaluate_policy protocol failure; payload:"; printf '%s\n' "$OUT"; exit 2 ;;
esac

case "$DECISION" in
    true)  echo "✓ smoke: authzen_evaluate decision=true forwarded from fake PDP" ;;
    false) echo "✗ smoke: authzen_evaluate decision=false (fake PDP returned true; mismatch)"; exit 1 ;;
    *)     echo "✗ smoke: authzen_evaluate no decision field. payload:"; printf '%s\n' "$OUT"; exit 2 ;;
esac
