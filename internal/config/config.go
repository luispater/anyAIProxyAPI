package config

import (
	"github.com/goccy/go-yaml"
	"os"
)

// AppConfig holds the application configuration.
type AppConfig struct {
	Version  string              `yaml:"version"`
	Debug    bool                `yaml:"debug"`
	Browser  AppConfigBrowser    `yaml:"browser"`
	Headless bool                `yaml:"headless"`
	ApiPort  string              `yaml:"api-port"`
	Instance []AppConfigInstance `yaml:"instance"`
}

type AppConfigBrowser struct {
	Use                     string   `yaml:"use"`
	CamoufoxPath            string   `yaml:"camoufox-path"`
	FingerprintChromiumPath string   `yaml:"fingerprint-chromium-path"`
	Args                    []string `yaml:"args"`
}
type AppConfigRunner struct {
	Init            string `yaml:"init"`
	ChatCompletions string `yaml:"chat_completions"`
	ContextCanceled string `yaml:"context_canceled"`
}
type AppConfigInstance struct {
	Name        string                `yaml:"name"`
	Adapter     string                `yaml:"adapter"`
	ProxyURL    string                `yaml:"proxy-url"`
	URL         string                `yaml:"url"`
	SniffPort   string                `yaml:"sniff-port"`
	SniffDomain string                `yaml:"sniff-domain"`
	Auth        AppConfigInstanceAuth `yaml:"auth"`
	Runner      AppConfigRunner       `yaml:"runner"`
}

type AppConfigInstanceAuth struct {
	File  string `yaml:"file"`
	Check string `yaml:"check"`
}

// LoadConfig loads configuration from environment variables or defaults.
func LoadConfig() (*AppConfig, error) {
	data, err := os.ReadFile("runner/main.yaml")
	if err != nil {
		return nil, err
	}

	var config AppConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
