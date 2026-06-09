package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/kevinmatthe/record_analysis/internal/llm"
	"github.com/kevinmatthe/record_analysis/internal/storage"
	"github.com/pelletier/go-toml/v2"
)

const EnvConfigPath = "RECORD_ANALYSIS_CONFIG_PATH"

type BaseConfig struct {
	ArkConfig        *ArkConfig           `json:"ark_config" yaml:"ark_config" toml:"ark_config"`
	MinioConfig      *storage.MinioConfig `json:"minio_config" yaml:"minio_config" toml:"minio_config"`
	DBConfig         *DBConfig            `json:"db_config" yaml:"db_config" toml:"db_config"`
	OpenSearchConfig *OpenSearchConfig    `json:"opensearch_config" yaml:"opensearch_config" toml:"opensearch_config"`
}

type DBConfig struct {
	Host                 string `json:"host" yaml:"host" toml:"host"`
	Port                 int    `json:"port" yaml:"port" toml:"port"`
	User                 string `json:"user" yaml:"user" toml:"user"`
	Password             string `json:"password" yaml:"password" toml:"password"`
	DBName               string `json:"dbname" yaml:"dbname" toml:"dbname"`
	SSLMode              string `json:"sslmode" yaml:"sslmode" toml:"sslmode"`
	Timezone             string `json:"timezone" yaml:"timezone" toml:"timezone"`
	ApplicationName      string `json:"application_name" yaml:"application_name" toml:"application_name"`
	ApplicationNameAlias string `json:"applicationName" yaml:"applicationName" toml:"applicationName"`
	SearchPath           string `json:"search_path" yaml:"search_path" toml:"search_path"`
}

type ArkConfig struct {
	APIKey              string `json:"api_key" yaml:"api_key" toml:"api_key"`
	BaseURL             string `json:"base_url" yaml:"base_url" toml:"base_url"`
	VisionModel         string `json:"vision_model" yaml:"vision_model" toml:"vision_model"`
	ReasoningModel      string `json:"reasoning_model" yaml:"reasoning_model" toml:"reasoning_model"`
	NormalModel         string `json:"normal_model" yaml:"normal_model" toml:"normal_model"`
	EmbeddingModel      string `json:"embedding_model" yaml:"embedding_model" toml:"embedding_model"`
	ChunkModel          string `json:"chunk_model" yaml:"chunk_model" toml:"chunk_model"`
	LiteModel           string `json:"lite_model" yaml:"lite_model" toml:"lite_model"`
	BatchEmbeddingModel string `json:"batch_embedding_model" yaml:"batch_embedding_model" toml:"batch_embedding_model"`
}

type OpenSearchConfig struct {
	Domain              string `json:"domain" yaml:"domain" toml:"domain"`
	User                string `json:"user" yaml:"user" toml:"user"`
	Password            string `json:"password" yaml:"password" toml:"password"`
	LarkCardActionIndex string `json:"lark_card_action_index" yaml:"lark_card_action_index" toml:"lark_card_action_index"`
	LarkChunkIndex      string `json:"lark_chunk_index" yaml:"lark_chunk_index" toml:"lark_chunk_index"`
	LarkMsgIndex        string `json:"lark_msg_index" yaml:"lark_msg_index" toml:"lark_msg_index"`
	RecordMessageIndex  string `json:"record_message_index" yaml:"record_message_index" toml:"record_message_index"`
	RecordSummaryIndex  string `json:"record_summary_index" yaml:"record_summary_index" toml:"record_summary_index"`
	RecordBranchIndex   string `json:"record_branch_index" yaml:"record_branch_index" toml:"record_branch_index"`
	Scheme              string `json:"scheme" yaml:"scheme" toml:"scheme"`
}

func LoadPath() string {
	if path := os.Getenv(EnvConfigPath); path != "" {
		return path
	}
	return ".dev/config.toml"
}

func LoadFile(path string) (*BaseConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &BaseConfig{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg.withDefaults(), nil
}

func (c *BaseConfig) withDefaults() *BaseConfig {
	if c.ArkConfig == nil {
		c.ArkConfig = &ArkConfig{}
	}
	if c.MinioConfig == nil {
		c.MinioConfig = &storage.MinioConfig{}
	}
	if c.DBConfig == nil {
		c.DBConfig = &DBConfig{}
	}
	if c.OpenSearchConfig == nil {
		c.OpenSearchConfig = &OpenSearchConfig{}
	}
	if c.DBConfig.ApplicationName == "" {
		c.DBConfig.ApplicationName = c.DBConfig.ApplicationNameAlias
	}
	return c
}

func (c *BaseConfig) SearchConfig() *OpenSearchConfig {
	if c == nil || c.OpenSearchConfig == nil || !c.OpenSearchConfig.Enabled() {
		return nil
	}
	cfg := *c.OpenSearchConfig
	if cfg.RecordMessageIndex == "" {
		cfg.RecordMessageIndex = cfg.LarkMsgIndex
	}
	if cfg.RecordMessageIndex == "" {
		cfg.RecordMessageIndex = "record_analysis_messages"
	}
	if cfg.RecordSummaryIndex == "" {
		cfg.RecordSummaryIndex = cfg.LarkChunkIndex
	}
	if cfg.RecordSummaryIndex == "" {
		cfg.RecordSummaryIndex = "record_analysis_summaries"
	}
	if cfg.RecordBranchIndex == "" {
		cfg.RecordBranchIndex = "record_analysis_branches"
	}
	return &cfg
}

func (c *OpenSearchConfig) Enabled() bool {
	return c != nil && strings.TrimSpace(c.Domain) != ""
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

func itoaDefault(value int, fallback int) string {
	if value == 0 {
		value = fallback
	}
	return strconv.Itoa(value)
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
