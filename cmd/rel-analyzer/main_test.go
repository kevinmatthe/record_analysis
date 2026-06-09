package main

import (
	"os"
	"strings"
	"testing"
)

func TestAnalyzeRequiresLLMBaseURLAndModelTogether(t *testing.T) {
	err := run([]string{
		"analyze",
		"../../records/常青藤/chat.csv",
		"--relationship-id", "rel_test",
		"--llm-base-url", "http://localhost:1234",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeLLMConfigUsesCLIOverrides(t *testing.T) {
	t.Setenv("CLI_KEY_ENV", "env-key")
	cfg := cliConfig{
		LLMBaseURL: "https://config.example",
		LLMModel:   "config-model",
		LLMAPIKey:  "config-key",
	}

	got, enabled, err := mergeLLMConfig(cfg, true, false, "https://cli.example", "cli-model", "CLI_KEY_ENV")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected enabled config")
	}
	if got.BaseURL != "https://cli.example" || got.Model != "cli-model" || got.APIKey != "env-key" {
		t.Fatalf("unexpected merged config: %+v", got)
	}
}

func TestMergeLLMConfigEnablesConfigOnlyLLMByDefault(t *testing.T) {
	cfg := cliConfig{
		LLMBaseURL: "https://config.example",
		LLMModel:   "config-model",
		LLMAPIKey:  "config-key",
	}

	_, enabled, err := mergeLLMConfig(cfg, false, false, "", "", "OPENAI_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("config-only LLM should be enabled by default")
	}
}

func TestMergeLLMConfigCanBeDisabledExplicitly(t *testing.T) {
	cfg := cliConfig{
		LLMBaseURL: "https://config.example",
		LLMModel:   "config-model",
		LLMAPIKey:  "config-key",
	}

	_, enabled, err := mergeLLMConfig(cfg, false, true, "", "", "OPENAI_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("explicitly disabled LLM should not run")
	}
}

func TestLoadCLIConfigCarriesMinioConfig(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	err := os.WriteFile(path, []byte(`
[minio_config]
ak = "ak"
sk = "sk"
bucket = "records"
[minio_config.internal]
endpoint = "internal:9000"
use_ssl = false
[minio_config.external]
endpoint = "external:9443"
use_ssl = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := loadCLIConfig(path, "normal")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MinioConfig == nil || cfg.MinioConfig.Bucket != "records" {
		t.Fatalf("minio config not loaded: %+v", cfg.MinioConfig)
	}
}

func TestParseOptionalTimeAcceptsDate(t *testing.T) {
	got, err := parseOptionalTime("2026-06-02")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Year() != 2026 || got.Month() != 6 || got.Day() != 2 {
		t.Fatalf("unexpected time: %v", got)
	}
}
