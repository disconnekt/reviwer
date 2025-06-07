package main

import (
	"os"
	"github.com/pelletier/go-toml/v2"
)

type LanguageConfig struct {
	Extension    string `toml:"extension"`
	ReviewPrompt string `toml:"review_prompt"`
	TestPrompt   string `toml:"test_prompt"`
}

type Config struct {
	Model       string                       `toml:"model"`
	ChunkSize   int                          `toml:"chunk_size"`
	LLMProvider string                       `toml:"llm_provider"`
	LLMModel    string                       `toml:"llm_model"`
	Languages   map[string]LanguageConfig    `toml:"languages"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = toml.Unmarshal(f, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
