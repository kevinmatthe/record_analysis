package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileReadsBetaGoStyleArkConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte(`
[ark_config]
api_key = "test-key"
normal_model = "doubao-normal"
reasoning_model = "doubao-reasoning"
lite_model = "doubao-lite"
base_url = "https://ark.example/api"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ArkConfig.APIKey != "test-key" {
		t.Fatalf("api key = %q", cfg.ArkConfig.APIKey)
	}
	if cfg.ArkConfig.NormalModel != "doubao-normal" {
		t.Fatalf("normal model = %q", cfg.ArkConfig.NormalModel)
	}
	if cfg.ArkConfig.BaseURL != "https://ark.example/api" {
		t.Fatalf("base url = %q", cfg.ArkConfig.BaseURL)
	}
}

func TestLoadFileReadsBetaGoStyleMinioConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte(`
[minio_config]
ak = "access-key"
sk = "secret-key"
expire_time = "168h"
bucket = "record-analysis"
    [minio_config.internal]
    endpoint = "minio.internal:9000"
    use_ssl = false
    [minio_config.external]
    endpoint = "minio.external:9443"
    use_ssl = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MinioConfig.AccessKey != "access-key" {
		t.Fatalf("access key = %q", cfg.MinioConfig.AccessKey)
	}
	if cfg.MinioConfig.Internal.Endpoint != "minio.internal:9000" {
		t.Fatalf("internal endpoint = %q", cfg.MinioConfig.Internal.Endpoint)
	}
	if !cfg.MinioConfig.External.UseSSL {
		t.Fatal("external use_ssl should be true")
	}
}

func TestLoadPathUsesRecordAnalysisConfigPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("RECORD_ANALYSIS_CONFIG_PATH", path)

	if got := LoadPath(); got != path {
		t.Fatalf("LoadPath() = %q, want %q", got, path)
	}
}

func TestLLMConfigFromArkDefaultsToNormalModel(t *testing.T) {
	cfg := &BaseConfig{
		ArkConfig: &ArkConfig{
			APIKey:         "test-key",
			NormalModel:    "doubao-normal",
			ReasoningModel: "doubao-reasoning",
			BaseURL:        "https://ark.example/api",
		},
	}

	llmCfg := cfg.LLMConfig("reasoning")

	if llmCfg.APIKey != "test-key" {
		t.Fatalf("api key = %q", llmCfg.APIKey)
	}
	if llmCfg.Model != "doubao-reasoning" {
		t.Fatalf("model = %q", llmCfg.Model)
	}
	if llmCfg.BaseURL != "https://ark.example/api" {
		t.Fatalf("base url = %q", llmCfg.BaseURL)
	}
}
