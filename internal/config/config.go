package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	defaultAPIHost  = "https://soul.ehomeease.com"
	defaultRelayURL = "wss://wsrelay.blurams.com:50843"
)

type Config struct {
	Email       string
	Password    string
	CameraID    string
	Transport   string
	Address     string
	APIHost     string
	RelayURL    string
	BearerToken string
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Email:       strings.TrimSpace(getenv("BLURAMS_EMAIL")),
		Password:    getenv("BLURAMS_PASSWORD"),
		CameraID:    strings.TrimSpace(getenv("BLURAMS_CAMERA_ID")),
		Transport:   valueOr(getenv("MCP_TRANSPORT"), "stdio"),
		Address:     valueOr(getenv("HTTP_ADDR"), ":8000"),
		APIHost:     strings.TrimRight(valueOr(getenv("BLURAMS_API_HOST"), defaultAPIHost), "/"),
		RelayURL:    valueOr(getenv("BLURAMS_RELAY_URL"), defaultRelayURL),
		BearerToken: getenv("MCP_BEARER_TOKEN"),
	}

	var missing []string
	if cfg.Email == "" {
		missing = append(missing, "BLURAMS_EMAIL")
	}
	if cfg.Password == "" {
		missing = append(missing, "BLURAMS_PASSWORD")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if cfg.Transport != "stdio" && cfg.Transport != "http" {
		return Config{}, errors.New("MCP_TRANSPORT must be stdio or http")
	}
	if cfg.Transport == "http" && len(cfg.BearerToken) < 32 {
		return Config{}, errors.New("HTTP mode requires MCP_BEARER_TOKEN with at least 32 characters")
	}
	return cfg, nil
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
