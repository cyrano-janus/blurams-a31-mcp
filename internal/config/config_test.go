package config

import "testing"

func TestLoadDefaultsToStdio(t *testing.T) {
	env := map[string]string{"BLURAMS_EMAIL": "a@example.com", "BLURAMS_PASSWORD": "secret"}
	cfg, err := load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "stdio" || cfg.Address != ":8000" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestHTTPRequiresLongBearerToken(t *testing.T) {
	env := map[string]string{
		"BLURAMS_EMAIL": "a@example.com", "BLURAMS_PASSWORD": "secret",
		"MCP_TRANSPORT": "http", "MCP_BEARER_TOKEN": "short",
	}
	_, err := load(func(key string) string { return env[key] })
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRejectsMissingCredentials(t *testing.T) {
	_, err := load(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected validation error")
	}
}
