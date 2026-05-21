package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Proxy      ProxyConfig              `yaml:"proxy"`
	Sinks      map[string]SinkConfig    `yaml:"sinks"`
	Middleware []MiddlewareConfig        `yaml:"middleware"`
	Privacy    PrivacyConfig            `yaml:"privacy"`
	Retention  RetentionConfig          `yaml:"retention"`
	Insights   InsightsConfig           `yaml:"insights"`
}

type ProxyConfig struct {
	Listen    string            `yaml:"listen"`     // default: "127.0.0.1:4191"
	Mode      string            `yaml:"mode"`       // "sidecar" | "shared"
	ServiceID string            `yaml:"service_id"` // identifies this agent in metrics
	Upstreams map[string]string `yaml:"upstreams"`  // provider name → base URL
}

type SinkConfig struct {
	Enabled bool           `yaml:"enabled"`
	Options map[string]any `yaml:"options"`
}

type MiddlewareConfig struct {
	Name    string         `yaml:"name"`
	Options map[string]any `yaml:"options"`
}

type PrivacyConfig struct {
	DataMinimisation bool   `yaml:"data_minimisation"`
	HashServiceID    bool   `yaml:"hash_service_id"` // default: true
	HashUser         bool   `yaml:"hash_user"`        // pseudonymise username before storage
	OrgSalt          string `yaml:"org_salt"`         // shared salt for user hashing; prefer TOKENMETER_ORG_SALT
	EncryptAtRest    bool   `yaml:"encrypt_at_rest"`
	EncryptionKey    string `yaml:"encryption_key"` // prefer TOKENMETER_ENCRYPTION_KEY env var
}

type RetentionConfig struct {
	Days int `yaml:"days"` // 0 = keep forever
}

type InsightsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	OllamaURL    string `yaml:"ollama_url"`    // default: http://localhost:11434
	Model        string `yaml:"model"`         // default: llama3.2:3b
	AutoGenerate string `yaml:"auto_generate"` // "daily" | "" (disabled)
	WindowDays   int    `yaml:"window_days"`   // default: 7
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

// Default returns a usable config without a file — for dev / start command.
func Default() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg
}

func applyDefaults(cfg *Config) {
	if cfg.Proxy.Listen == "" {
		cfg.Proxy.Listen = "127.0.0.1:4191"
	}
	if cfg.Proxy.Mode == "" {
		cfg.Proxy.Mode = "sidecar"
	}
	if cfg.Proxy.Upstreams == nil {
		cfg.Proxy.Upstreams = map[string]string{}
	}
	if cfg.Proxy.Upstreams["anthropic"] == "" {
		cfg.Proxy.Upstreams["anthropic"] = "https://api.anthropic.com"
	}
	if cfg.Proxy.Upstreams["openai"] == "" {
		cfg.Proxy.Upstreams["openai"] = "https://api.openai.com"
	}
	if cfg.Retention.Days == 0 {
		cfg.Retention.Days = 90
	}
	cfg.Privacy.HashServiceID = true // always default to hashing
	if salt := os.Getenv("TOKENMETER_ORG_SALT"); salt != "" {
		cfg.Privacy.OrgSalt = salt
	}
	if cfg.Insights.OllamaURL == "" {
		cfg.Insights.OllamaURL = "http://localhost:11434"
	}
	if cfg.Insights.Model == "" {
		cfg.Insights.Model = "llama3.2:3b"
	}
	if cfg.Insights.WindowDays == 0 {
		cfg.Insights.WindowDays = 7
	}
}
