package main

import (
	"strings"
	"testing"
	"time"
)

// env builds a getenv function over a map, so config tests do not mutate the
// process environment and can run in parallel.
func env(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

func TestLoadConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg, err := loadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PDPURL != "" || cfg.PDPToken != "" {
		t.Fatal("an unconfigured server must not invent a PDP")
	}
	if cfg.PDPTimeout != defaultPDPTimeout {
		t.Fatalf("PDPTimeout = %s", cfg.PDPTimeout)
	}
	if cfg.RegoTimeout != defaultRegoTimeout {
		t.Fatalf("RegoTimeout = %s", cfg.RegoTimeout)
	}
	if cfg.PDPMaxBytes != defaultPDPMaxBytes {
		t.Fatalf("PDPMaxBytes = %d", cfg.PDPMaxBytes)
	}
	if cfg.MaxArgBytes != defaultMaxArgBytes {
		t.Fatalf("MaxArgBytes = %d", cfg.MaxArgBytes)
	}
	// The sandbox is the default. If this ever flips, a model-supplied policy
	// gets network access on every install that never set the variable.
	if cfg.AllowNetworkBuiltins {
		t.Fatal("network built-ins must be off unless explicitly enabled")
	}
}

func TestLoadConfig_ReadsEverything(t *testing.T) {
	t.Parallel()

	cfg, err := loadConfig(env(map[string]string{
		envPDPURL:        "https://pdp.example.com/access/v1/evaluation",
		envPDPToken:      "s3cret",
		envPDPTimeout:    "30s",
		envPDPMaxBytes:   "2048",
		envRegoTimeout:   "1m",
		envMaxArgBytes:   "4096",
		envAllowNetBuilt: "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PDPURL != "https://pdp.example.com/access/v1/evaluation" {
		t.Fatalf("PDPURL = %q", cfg.PDPURL)
	}
	if cfg.PDPToken != "s3cret" {
		t.Fatalf("PDPToken = %q", cfg.PDPToken)
	}
	if cfg.PDPTimeout != 30*time.Second {
		t.Fatalf("PDPTimeout = %s", cfg.PDPTimeout)
	}
	if cfg.RegoTimeout != time.Minute {
		t.Fatalf("RegoTimeout = %s", cfg.RegoTimeout)
	}
	if cfg.PDPMaxBytes != 2048 || cfg.MaxArgBytes != 4096 {
		t.Fatalf("byte limits = %d / %d", cfg.PDPMaxBytes, cfg.MaxArgBytes)
	}
	if !cfg.AllowNetworkBuiltins {
		t.Fatal("AllowNetworkBuiltins = false")
	}
}

// A malformed bound must stop the server. Falling back to the default would
// leave an operator believing a limit is in force that is not.
func TestLoadConfig_RejectsMalformedValues(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"timeout without unit": {envPDPTimeout: "30"},
		"timeout not a number": {envPDPTimeout: "soon"},
		"negative timeout":     {envRegoTimeout: "-5s"},
		"zero timeout":         {envRegoTimeout: "0s"},
		"size not a number":    {envPDPMaxBytes: "lots"},
		"negative size":        {envMaxArgBytes: "-1"},
		"zero size":            {envPDPMaxBytes: "0"},
		"bool not a bool":      {envAllowNetBuilt: "yes please"},
	}

	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg, err := loadConfig(env(vars))
			if err == nil {
				t.Fatalf("loadConfig accepted %v and produced %+v", vars, cfg)
			}
			for k := range vars {
				if !strings.Contains(err.Error(), k) {
					t.Fatalf("error %q does not name the variable %q", err, k)
				}
			}
		})
	}
}

func TestLoadConfig_AcceptsBoolVocabulary(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]bool{
		"true": true, "TRUE": true, "1": true, "t": true,
		"false": false, "0": false, "": false,
	} {
		cfg, err := loadConfig(env(map[string]string{envAllowNetBuilt: raw}))
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if cfg.AllowNetworkBuiltins != want {
			t.Fatalf("%q gave %v, want %v", raw, cfg.AllowNetworkBuiltins, want)
		}
	}
}

func TestCapabilities_FollowsTheFlag(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	if cfg.capabilities() != sandboxCapabilities {
		t.Fatal("the default must be the sandboxed set")
	}
	cfg.AllowNetworkBuiltins = true
	if cfg.capabilities() != baseCapabilities {
		t.Fatal("the opt-out must return the full set")
	}
}
