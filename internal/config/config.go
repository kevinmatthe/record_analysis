package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/storage"
)

const EnvConfigPath = "RECORD_ANALYSIS_CONFIG_PATH"

type BaseConfig struct {
	ArkConfig   *ArkConfig           `json:"ark_config" toml:"ark_config"`
	MinioConfig *storage.MinioConfig `json:"minio_config" toml:"minio_config"`
	DBConfig    *DBConfig            `json:"db_config" toml:"db_config"`
}

type DBConfig struct {
	Host            string `json:"host" toml:"host"`
	Port            int    `json:"port" toml:"port"`
	User            string `json:"user" toml:"user"`
	Password        string `json:"password" toml:"password"`
	DBName          string `json:"dbname" toml:"dbname"`
	SSLMode         string `json:"sslmode" toml:"sslmode"`
	Timezone        string `json:"timezone" toml:"timezone"`
	ApplicationName string `json:"application_name" toml:"application_name"`
	SearchPath      string `json:"search_path" toml:"search_path"`
}

type ArkConfig struct {
	APIKey              string `json:"api_key" toml:"api_key"`
	BaseURL             string `json:"base_url" toml:"base_url"`
	VisionModel         string `json:"vision_model" toml:"vision_model"`
	ReasoningModel      string `json:"reasoning_model" toml:"reasoning_model"`
	NormalModel         string `json:"normal_model" toml:"normal_model"`
	EmbeddingModel      string `json:"embedding_model" toml:"embedding_model"`
	ChunkModel          string `json:"chunk_model" toml:"chunk_model"`
	LiteModel           string `json:"lite_model" toml:"lite_model"`
	BatchEmbeddingModel string `json:"batch_embedding_model" toml:"batch_embedding_model"`
}

func LoadPath() string {
	if path := os.Getenv(EnvConfigPath); path != "" {
		return path
	}
	return ".dev/config.toml"
}

func LoadFile(path string) (*BaseConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &BaseConfig{ArkConfig: &ArkConfig{}, MinioConfig: &storage.MinioConfig{}, DBConfig: &DBConfig{}}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := stripComment(strings.TrimSpace(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			continue
		}
		switch section {
		case "ark_config":
			assignArkConfig(cfg.ArkConfig, key, value)
		case "minio_config":
			assignMinioConfig(cfg.MinioConfig, key, value)
		case "minio_config.internal":
			assignEndpointConfig(&cfg.MinioConfig.Internal, key, value)
		case "minio_config.external":
			assignEndpointConfig(&cfg.MinioConfig.External, key, value)
		case "db_config":
			assignDBConfig(cfg.DBConfig, key, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *BaseConfig) HasDBConfig() bool {
	return c != nil && c.DBConfig != nil && c.DBConfig.Host != "" && c.DBConfig.User != "" && c.DBConfig.DBName != ""
}

func (c *BaseConfig) DBDSN() string {
	if !c.HasDBConfig() {
		return ""
	}
	cfg := c.DBConfig
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	parts := []string{
		"host=" + cfg.Host,
		"port=" + itoaDefault(cfg.Port, 5432),
		"user=" + cfg.User,
		"password=" + cfg.Password,
		"dbname=" + cfg.DBName,
		"sslmode=" + sslMode,
		"TimeZone=" + timezone,
	}
	if cfg.ApplicationName != "" {
		parts = append(parts, "application_name="+cfg.ApplicationName)
	}
	if cfg.SearchPath != "" {
		parts = append(parts, "search_path="+cfg.SearchPath)
	}
	return strings.Join(parts, " ")
}

func assignDBConfig(cfg *DBConfig, key string, value string) {
	switch key {
	case "host":
		cfg.Host = value
	case "port":
		cfg.Port = atoiDefault(value, 0)
	case "user":
		cfg.User = value
	case "password":
		cfg.Password = value
	case "dbname":
		cfg.DBName = value
	case "sslmode":
		cfg.SSLMode = value
	case "timezone":
		cfg.Timezone = value
	case "application_name", "applicationName":
		cfg.ApplicationName = value
	case "search_path":
		cfg.SearchPath = value
	}
}

func itoaDefault(value int, fallback int) string {
	if value == 0 {
		value = fallback
	}
	return strconv.Itoa(value)
}

func atoiDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (c *BaseConfig) ObjectStoreConfig() *storage.MinioConfig {
	if c == nil || c.MinioConfig == nil || !c.MinioConfig.Enabled() {
		return nil
	}
	return c.MinioConfig
}

func LoadDefault() (*BaseConfig, error) {
	return LoadFile(LoadPath())
}

func assignMinioConfig(cfg *storage.MinioConfig, key string, value string) {
	switch key {
	case "ak", "ak_id", "access_key":
		cfg.AccessKey = value
	case "sk", "secret_key":
		cfg.SecretKey = value
	case "expire_time":
		cfg.ExpireTime = value
	case "bucket":
		cfg.Bucket = value
	}
}

func assignEndpointConfig(cfg *storage.EndpointConfig, key string, value string) {
	switch key {
	case "endpoint":
		cfg.Endpoint = value
	case "use_ssl":
		cfg.UseSSL = value == "true"
	}
}

func (c *BaseConfig) LLMConfig(profile string) llm.OpenAICompatibleConfig {
	if c == nil || c.ArkConfig == nil {
		return llm.OpenAICompatibleConfig{}
	}
	model := c.ArkConfig.NormalModel
	switch profile {
	case "reasoning":
		if c.ArkConfig.ReasoningModel != "" {
			model = c.ArkConfig.ReasoningModel
		}
	case "lite":
		if c.ArkConfig.LiteModel != "" {
			model = c.ArkConfig.LiteModel
		}
	case "vision":
		if c.ArkConfig.VisionModel != "" {
			model = c.ArkConfig.VisionModel
		}
	case "chunk":
		if c.ArkConfig.ChunkModel != "" {
			model = c.ArkConfig.ChunkModel
		}
	}
	return llm.OpenAICompatibleConfig{
		BaseURL: c.ArkConfig.BaseURL,
		APIKey:  c.ArkConfig.APIKey,
		Model:   model,
	}
}

func assignArkConfig(cfg *ArkConfig, key string, value string) {
	switch key {
	case "api_key":
		cfg.APIKey = value
	case "base_url":
		cfg.BaseURL = value
	case "vision_model":
		cfg.VisionModel = value
	case "reasoning_model":
		cfg.ReasoningModel = value
	case "normal_model":
		cfg.NormalModel = value
	case "embedding_model":
		cfg.EmbeddingModel = value
	case "chunk_model":
		cfg.ChunkModel = value
	case "lite_model":
		cfg.LiteModel = value
	case "batch_embedding_model":
		cfg.BatchEmbeddingModel = value
	}
}

func parseAssignment(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"`)
	return key, value, key != ""
}

func stripComment(line string) string {
	inQuote := false
	for index, r := range line {
		if r == '"' {
			inQuote = !inQuote
		}
		if r == '#' && !inQuote {
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}
