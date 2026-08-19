## What this changes

<!-- One or two sentences. What is different after this merges. -->

## Why

<!-- The problem. For a decision-path change, say which way it was wrong before. -->

## Checklist

- [ ] `make lint && make test && make smoke` pass locally
- [ ] A test fails without this change
- [ ] `CHANGELOG.md` updated under `## Unreleased`, if this is user-visible

## If this touches a decision path

<!-- Delete this section if it does not. -->

- [ ] A failure to obtain a decision is still distinguishable from a deny
- [ ] Batch decisions still carry their index, and a short response is still reported as truncated
- [ ] Anything echoed back to the model is still bounded

## If this touches the Rego sandbox

<!-- Delete this section if it does not. -->

- [ ] `http.send`, `net.lookup_ip_addr` and `opa.runtime` are still removed by default
- [ ] The evaluation deadline still applies
