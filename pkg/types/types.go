package types

import (
	"context"
	"time"
)

// LanguageConfig holds configuration for a specific programming language
type LanguageConfig struct {
	Extension    string
	ReviewPrompt string
	TestPrompt   string
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// ReviewConfig holds review-specific configuration
type ReviewConfig struct {
	ChunkSize    int
	MaxRetries   int
	ChunkTimeout time.Duration
	WriteTests   bool
	KeepTests    bool
}

// Config represents the main configuration structure
type Config struct {
	LLM       LLMConfig
	Review    ReviewConfig
	Languages map[string]LanguageConfig
}

// NewDefaultConfig returns a configuration with sensible defaults
func NewDefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
			BaseURL:  "http://127.0.0.1:1234/v1",
		},
		Review: ReviewConfig{
			ChunkSize:    1000,
			MaxRetries:   3, // Уменьшено для более быстрого фейла при проблемах
			ChunkTimeout: 15 * time.Minute, // Увеличено для соответствия HTTP таймауту
			WriteTests:   false, // Отключено по умолчанию для ускорения работы с LM Studio
			KeepTests:    false,
		},
		Languages: map[string]LanguageConfig{
			"go": {
				Extension:    ".go",
				ReviewPrompt: getGoReviewPrompt(),
				TestPrompt:   getGoTestPrompt(),
			},
			"php": {
				Extension:    ".php",
				ReviewPrompt: getPHPReviewPrompt(),
				TestPrompt:   getPHPTestPrompt(),
			},
		},
	}
}

// FailedChunk represents a chunk that failed processing
type FailedChunk struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// ReviewMode represents different review modes
type ReviewMode string

const (
	ModeDiffUncommitted ReviewMode = "diff-uncommitted"
	ModeDiffBranch      ReviewMode = "diff-branch"
	ModeReviewProject   ReviewMode = "review-project"
	ModeReviewFile      ReviewMode = "review-file"
)

// ReviewOptions holds options for review operations
type ReviewOptions struct {
	Mode             ReviewMode
	Dir              string
	File             string
	Base             string
	WriteTests       bool
	KeepTests        bool
	MaxRetries       int
	ChunkTimeout     time.Duration
	FailedChunksFile string
	ResumeFailed     bool
}

// DiffProvider defines interface for git diff and chunking operations
type DiffProvider interface {
	GetUncommittedDiff(dir string) (string, error)
	GetBranchDiff(dir, base string) (string, error)
	ChunkDiff(diff string, chunkSize int) []string
	GetProjectChunks(dir string, chunkSize int, extensions []string) ([]string, error)
	GetFileChunks(path string, chunkSize int) ([]string, error)
}

// LLMProvider defines interface for LLM operations
type LLMProvider interface {
	ReviewChunk(ctx context.Context, prompt, code, lang string) (string, error)
	GenerateUnitTests(ctx context.Context, prompt, code, lang string) (string, error)
	HealthCheck(ctx context.Context) error
}

// ConfigProvider defines interface for configuration operations
type ConfigProvider interface {
	Load(path string) (*Config, error)
}

// getGoReviewPrompt returns optimized Go code review prompt
func getGoReviewPrompt() string {
	return `Analyze this Go code for issues. Be concise and factual. Double-check your findings.

Focus on:
- Bugs and logic errors
- Performance issues
- Security vulnerabilities
- Code quality problems

Format: Issue type | Line/function | Problem | Solution
Example: Bug | line 15 | nil pointer risk | add nil check

Only report actual issues. No general advice.`
}

// getGoTestPrompt returns optimized Go test generation prompt
func getGoTestPrompt() string {
	return `Generate Go unit tests for this code. Be precise and comprehensive.

Requirements:
- Test all public functions
- Include edge cases and error conditions
- Use table-driven tests where appropriate
- Mock external dependencies
- Follow Go testing conventions

Generate only the test code. No explanations.`
}

// getPHPReviewPrompt returns optimized PHP code review prompt
func getPHPReviewPrompt() string {
	return `Analyze this PHP code for issues. Be concise and factual. Double-check your findings.

Focus on:
- Bugs and logic errors
- Security vulnerabilities (SQL injection, XSS, etc.)
- Performance issues
- PSR compliance

Format: Issue type | Line/function | Problem | Solution
Example: Security | line 23 | SQL injection risk | use prepared statements

Only report actual issues. No general advice.`
}

// getPHPTestPrompt returns optimized PHP test generation prompt
func getPHPTestPrompt() string {
	return `Generate PHPUnit tests for this code. Be precise and comprehensive.

Requirements:
- Test all public methods
- Include edge cases and error conditions
- Mock dependencies using PHPUnit mocks
- Follow PHPUnit conventions
- Test both success and failure scenarios

Generate only the test code. No explanations.`
}
