package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Environment variable names. AUTHZEN_PDP_URL and AUTHZEN_PDP_TOKEN predate the
// rest and are load-bearing in existing MCP client configs, so they keep their
// bare names. Everything else is prefixed by what it configures: AUTHZEN_PDP_
// for the PDP client, MCP_OPA_ for policy evaluation, MCP_ for the server.
const (
	envPDPURL        = "AUTHZEN_PDP_URL"
	envPDPToken      = "AUTHZEN_PDP_TOKEN"
	envPDPTimeout    = "AUTHZEN_PDP_TIMEOUT"
	envPDPMaxBytes   = "AUTHZEN_PDP_MAX_RESPONSE_BYTES"
	envRegoTimeout   = "MCP_OPA_EVAL_TIMEOUT"
	envMaxArgBytes   = "MCP_MAX_ARG_BYTES"
	envAllowNetBuilt = "MCP_OPA_ALLOW_NETWORK_BUILTINS"
)

// Defaults. Every one of these is a bound on work the *model* asked for, so the
// question they answer is "how much can one bad tool call cost", not "what is
// generous". A PDP that has not answered in ten seconds is down; a Rego module
// that has not converged in five is looping.
const (
	defaultPDPTimeout  = 10 * time.Second
	defaultPDPMaxBytes = 1 << 20 // 1 MiB
	defaultRegoTimeout = 5 * time.Second
	// Every structured argument — Rego source, the JSON documents around it,
	// and the AuthZEN entities — arrives inside a JSON-RPC message the model
	// composed. A megabyte is far past any hand-written policy and still cheap
	// to reject.
	defaultMaxArgBytes = 1 << 20 // 1 MiB
)

// config is the process-wide configuration, read once at startup rather than
// per tool call. A stdio MCP server is a subprocess with a fixed environment:
// re-reading os.Getenv on every call would only make behaviour depend on when
// the call happened.
type config struct {
	// PDPURL is the default AuthZEN Access Evaluation endpoint. May be empty,
	// in which case every authzen_* call must supply pdp_url itself.
	PDPURL string
	// PDPToken is sent as the Authorization header. Already-prefixed values
	// ("Bearer x", "Basic y") are passed through untouched.
	PDPToken string
	// PDPTimeout bounds a single PDP round trip.
	PDPTimeout time.Duration
	// PDPMaxBytes bounds how much of a PDP response is read into memory.
	PDPMaxBytes int64
	// RegoTimeout bounds a single in-process Rego evaluation.
	RegoTimeout time.Duration
	// MaxArgBytes bounds the size of a single structured argument.
	MaxArgBytes int
	// AllowNetworkBuiltins re-enables the Rego built-ins that can reach the
	// network or the host from inside a model-supplied policy. Off by default;
	// see opa_capabilities.go for why.
	AllowNetworkBuiltins bool
}

// loadConfig reads the environment. It returns an error rather than falling
// back to a default on a malformed value: silently ignoring
// AUTHZEN_PDP_TIMEOUT=30 (no unit) would leave an operator believing a bound
// is in place that is not.
func loadConfig(getenv func(string) string) (*config, error) {
	c := &config{
		PDPURL:      getenv(envPDPURL),
		PDPToken:    getenv(envPDPToken),
		PDPTimeout:  defaultPDPTimeout,
		PDPMaxBytes: defaultPDPMaxBytes,
		RegoTimeout: defaultRegoTimeout,
		MaxArgBytes: defaultMaxArgBytes,
	}

	var err error
	if c.PDPTimeout, err = durationEnv(getenv, envPDPTimeout, defaultPDPTimeout); err != nil {
		return nil, err
	}
	if c.RegoTimeout, err = durationEnv(getenv, envRegoTimeout, defaultRegoTimeout); err != nil {
		return nil, err
	}

	n, err := int64Env(getenv, envPDPMaxBytes, defaultPDPMaxBytes)
	if err != nil {
		return nil, err
	}
	c.PDPMaxBytes = n

	n, err = int64Env(getenv, envMaxArgBytes, defaultMaxArgBytes)
	if err != nil {
		return nil, err
	}
	c.MaxArgBytes = int(n)

	if c.AllowNetworkBuiltins, err = boolEnv(getenv, envAllowNetBuilt); err != nil {
		return nil, err
	}

	return c, nil
}

func durationEnv(getenv func(string) string, name string, def time.Duration) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a duration (want e.g. \"10s\", \"1m\"): %w", name, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %q", name, raw)
	}
	return d, nil
}

func int64Env(getenv func(string) string, name string, def int64) (int64, error) {
	raw := getenv(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not an integer: %w", name, raw, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %q", name, raw)
	}
	return n, nil
}

// boolEnv accepts the strconv.ParseBool vocabulary. An unset or empty value is
// false, which keeps "not configured" and "configured off" the same thing for
// every flag here — they all default to the safe side.
func boolEnv(getenv func(string) string, name string) (bool, error) {
	raw := getenv(name)
	if raw == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %q is not a boolean (want \"true\" or \"false\"): %w", name, raw, err)
	}
	return b, nil
}

// osGetenv is the production source for loadConfig.
func osGetenv(name string) string { return os.Getenv(name) }
