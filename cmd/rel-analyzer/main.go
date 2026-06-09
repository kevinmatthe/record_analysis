package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/kevinmatthe/record_analysis/internal/config"
	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/search"
	"github.com/kevinmatthe/record_analysis/internal/server"
	"github.com/kevinmatthe/record_analysis/internal/service"
	"github.com/kevinmatthe/record_analysis/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: rel-analyzer analyze <chat-file> --relationship-id rel_001")
	}
	switch args[0] {
	case "analyze":
		return runAnalyze(args[1:])
	case "serve":
		return runServe(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runAnalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("analyze requires one chat file")
	}
	chatFile := ""
	if !strings.HasPrefix(args[0], "-") {
		chatFile = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	relationshipID := fs.String("relationship-id", "", "relationship id")
	output := fs.String("output", ".data/reports/report.md", "report output path")
	jsonOutput := fs.String("json-output", "", "optional JSON analysis output path")
	objectRoot := fs.String("object-root", ".data/objects", "local object store root")
	includeSystem := fs.Bool("include-system", false, "include system messages during parsing")
	fromText := fs.String("from", "", "inclusive period start, YYYY-MM-DD or YYYY-MM-DD HH:MM:SS")
	toText := fs.String("to", "", "exclusive period end, YYYY-MM-DD or YYYY-MM-DD HH:MM:SS")
	maxLLMMessages := fs.Int("max-llm-messages", 500, "maximum messages allowed when LLM extraction is enabled")
	configPath := fs.String("config", "", "config file path; defaults to RECORD_ANALYSIS_CONFIG_PATH or .dev/config.toml when present")
	llmProfile := fs.String("llm-profile", "normal", "LLM profile from ark_config: normal, reasoning, lite, vision, chunk")
	enableLLM := fs.Bool("enable-llm", false, "enable structured LLM extraction using config or LLM flags")
	disableLLM := fs.Bool("disable-llm", false, "disable structured LLM extraction even when config is present")
	llmBaseURL := fs.String("llm-base-url", "", "OpenAI-compatible base URL; enables structured LLM extraction when set with --llm-model")
	llmModel := fs.String("llm-model", "", "OpenAI-compatible model name")
	llmAPIKeyEnv := fs.String("llm-api-key-env", "OPENAI_API_KEY", "environment variable containing the LLM API key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chatFile == "" && fs.NArg() > 0 {
		chatFile = fs.Arg(0)
	}
	if *relationshipID == "" {
		return fmt.Errorf("--relationship-id is required")
	}
	if chatFile == "" {
		return fmt.Errorf("analyze requires one chat file")
	}
	cfg, err := loadCLIConfig(*configPath, *llmProfile)
	if err != nil {
		return err
	}
	objectStore, err := buildObjectStore(cfg, *objectRoot)
	if err != nil {
		return err
	}
	svc := service.NewChatAnalysisService(objectStore, filepath.Dir(*output))
	llmCfg, enabled, err := mergeLLMConfig(cfg, *enableLLM || *llmBaseURL != "" || *llmModel != "", *disableLLM, *llmBaseURL, *llmModel, *llmAPIKeyEnv)
	if err != nil {
		return err
	}
	if enabled {
		llmCfg.Logf = func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
		fmt.Fprintf(os.Stderr, "llm enabled base_url=%s model=%s\n", llmCfg.BaseURL, llmCfg.Model)
		svc.WithExtractor(llm.NewOpenAICompatibleExtractor(llmCfg))
	}
	from, err := parseOptionalTime(*fromText)
	if err != nil {
		return err
	}
	to, err := parseOptionalTime(*toText)
	if err != nil {
		return err
	}
	result, err := svc.UploadAndAnalyzeWithOptions(chatFile, *relationshipID, service.UploadAnalyzeOptions{
		IncludeSystem:  *includeSystem,
		From:           from,
		To:             to,
		MaxLLMMessages: *maxLLMMessages,
	})
	if err != nil {
		return err
	}
	if result.ReportPath != *output {
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*output, []byte(result.Analysis.Report.Markdown), 0o644); err != nil {
			return err
		}
	}
	if *jsonOutput != "" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*jsonOutput), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*jsonOutput, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Println(*output)
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	outputRoot := fs.String("output-root", ".data/reports", "report output root")
	objectRoot := fs.String("object-root", ".data/objects", "local object store root")
	historyPath := fs.String("history-path", ".data/analysis/index.jsonl", "analysis history JSONL path")
	maxUploadMB := fs.Int("max-upload-mb", 64, "maximum uploaded chat file size in MiB")
	maxLLMMessages := fs.Int("max-llm-messages", 500, "default maximum messages allowed when LLM extraction is enabled")
	authUsername := fs.String("auth-username", os.Getenv("RECORD_ANALYSIS_AUTH_USERNAME"), "web API login username")
	authPassword := fs.String("auth-password", os.Getenv("RECORD_ANALYSIS_AUTH_PASSWORD"), "web API login password")
	corsOrigin := fs.String("cors-origin", "http://localhost:5173", "allowed CORS origin for the separated WebUI")
	configPath := fs.String("config", "", "config file path; defaults to RECORD_ANALYSIS_CONFIG_PATH or .dev/config.toml when present")
	llmProfile := fs.String("llm-profile", "normal", "LLM profile from ark_config: normal, reasoning, lite, vision, chunk")
	enableLLM := fs.Bool("enable-llm", false, "enable structured LLM extraction using config or LLM flags")
	disableLLM := fs.Bool("disable-llm", false, "disable structured LLM extraction even when config is present")
	llmBaseURL := fs.String("llm-base-url", "", "OpenAI-compatible base URL; enables structured LLM extraction when set with --llm-model")
	llmModel := fs.String("llm-model", "", "OpenAI-compatible model name")
	llmAPIKeyEnv := fs.String("llm-api-key-env", "OPENAI_API_KEY", "environment variable containing the LLM API key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := buildAnalysisService(*outputRoot, *objectRoot, *configPath, *llmProfile, *enableLLM || *llmBaseURL != "" || *llmModel != "", *disableLLM, *llmBaseURL, *llmModel, *llmAPIKeyEnv)
	if err != nil {
		return err
	}
	cfg, err := loadCLIConfig(*configPath, *llmProfile)
	if err != nil {
		return err
	}
	var pgStore *server.PostgresStore
	if cfg.DBDSN != "" {
		pgStore, err = server.NewPostgresStore(cfg.DBDSN)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "postgres enabled for record_analysis persistence")
		if *authUsername != "" && *authPassword != "" {
			if err := pgStore.EnsureUser(*authUsername, *authPassword); err != nil {
				return err
			}
		}
		svc.WithHistoryStore(pgStore)
	} else {
		fmt.Fprintln(os.Stderr, "postgres disabled: db_config missing; registration/password changes are unavailable")
		svc.WithHistoryStore(service.NewJSONLAnalysisHistoryStore(*historyPath))
	}
	handler := server.NewHandler(svc, server.Options{
		MaxUploadBytes: int64(*maxUploadMB) << 20,
		MaxLLMMessages: *maxLLMMessages,
		AuthUsername:   *authUsername,
		AuthPassword:   *authPassword,
		AllowedOrigin:  *corsOrigin,
		Store:          pgStore,
		Indexer:        buildSearchIndexer(cfg),
	})
	fmt.Println("listening on " + *addr)
	return http.ListenAndServe(*addr, handler)
}

func buildAnalysisService(outputRoot string, objectRoot string, configPath string, llmProfile string, enableLLM bool, disableLLM bool, llmBaseURL string, llmModel string, llmAPIKeyEnv string) (*service.ChatAnalysisService, error) {
	cfg, err := loadCLIConfig(configPath, llmProfile)
	if err != nil {
		return nil, err
	}
	objectStore, err := buildObjectStore(cfg, objectRoot)
	if err != nil {
		return nil, err
	}
	svc := service.NewChatAnalysisService(objectStore, outputRoot)
	llmCfg, enabled, err := mergeLLMConfig(cfg, enableLLM, disableLLM, llmBaseURL, llmModel, llmAPIKeyEnv)
	if err != nil {
		return nil, err
	}
	if enabled {
		llmCfg.Logf = func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
		fmt.Fprintf(os.Stderr, "llm enabled base_url=%s model=%s\n", llmCfg.BaseURL, llmCfg.Model)
		svc.WithExtractor(llm.NewOpenAICompatibleExtractor(llmCfg))
	}
	return svc, nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time %q; use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS", value)
}

func buildObjectStore(cfg cliConfig, objectRoot string) (storage.ObjectStore, error) {
	if cfg.MinioConfig == nil {
		return storage.NewLocalObjectStore(objectRoot), nil
	}
	store, err := storage.NewMinioObjectStore(*cfg.MinioConfig)
	if err != nil {
		return nil, err
	}
	return store, nil
}

type cliConfig struct {
	LLMBaseURL       string
	LLMModel         string
	LLMAPIKey        string
	MinioConfig      *storage.MinioConfig
	DBDSN            string
	OpenSearchConfig *appconfig.OpenSearchConfig
}

func loadCLIConfig(path string, profile string) (cliConfig, error) {
	if path == "" {
		path = appconfig.LoadPath()
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cliConfig{}, nil
		}
		return cliConfig{}, err
	}
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return cliConfig{}, err
	}
	llmCfg := cfg.LLMConfig(profile)
	return cliConfig{
		LLMBaseURL:       llmCfg.BaseURL,
		LLMModel:         llmCfg.Model,
		LLMAPIKey:        llmCfg.APIKey,
		MinioConfig:      cfg.ObjectStoreConfig(),
		DBDSN:            cfg.DBDSN(),
		OpenSearchConfig: cfg.SearchConfig(),
	}, nil
}

func buildSearchIndexer(cfg cliConfig) search.Indexer {
	if cfg.OpenSearchConfig == nil {
		return search.NoopIndexer{}
	}
	indexer, err := search.NewOpenSearchIndexer(search.Config{
		Domain:             cfg.OpenSearchConfig.Domain,
		User:               cfg.OpenSearchConfig.User,
		Password:           cfg.OpenSearchConfig.Password,
		RecordMessageIndex: cfg.OpenSearchConfig.RecordMessageIndex,
		RecordSummaryIndex: cfg.OpenSearchConfig.RecordSummaryIndex,
		RecordBranchIndex:  cfg.OpenSearchConfig.RecordBranchIndex,
		Scheme:             cfg.OpenSearchConfig.Scheme,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "opensearch disabled: %v\n", err)
		return search.NoopIndexer{}
	}
	fmt.Fprintf(os.Stderr, "opensearch enabled domain=%s message_index=%s summary_index=%s branch_index=%s\n",
		cfg.OpenSearchConfig.Domain,
		cfg.OpenSearchConfig.RecordMessageIndex,
		cfg.OpenSearchConfig.RecordSummaryIndex,
		cfg.OpenSearchConfig.RecordBranchIndex,
	)
	return indexer
}

func mergeLLMConfig(cfg cliConfig, enable bool, disable bool, baseURL string, model string, apiKeyEnv string) (llm.OpenAICompatibleConfig, bool, error) {
	if baseURL != "" {
		cfg.LLMBaseURL = baseURL
	}
	if model != "" {
		cfg.LLMModel = model
	}
	if apiKey := os.Getenv(apiKeyEnv); apiKey != "" {
		cfg.LLMAPIKey = apiKey
	}
	if disable {
		return llm.OpenAICompatibleConfig{}, false, nil
	}
	if cfg.LLMBaseURL == "" || cfg.LLMModel == "" {
		if !enable && cfg.LLMBaseURL == "" && cfg.LLMModel == "" {
			return llm.OpenAICompatibleConfig{}, false, nil
		}
		return llm.OpenAICompatibleConfig{}, false, fmt.Errorf("--llm-base-url and --llm-model must be set together or provided by config")
	}
	return llm.OpenAICompatibleConfig{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
	}, true, nil
}
