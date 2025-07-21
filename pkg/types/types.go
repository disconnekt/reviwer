package types

import (
	"context"
	"time"
)

type LanguageConfig struct {
	Extension    string
	ReviewPrompt string
	TestPrompt   string
}

type LLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

type ReviewConfig struct {
	ChunkSize    int
	MaxRetries   int
	ChunkTimeout time.Duration
	WriteTests   bool
	KeepTests    bool
}

type Config struct {
	LLM       LLMConfig
	Review    ReviewConfig
	Languages map[string]LanguageConfig
}

func NewDefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: "openai", // openai, lmstudio, claude, mistral, groq
			Model:    "gpt-4",
			BaseURL:  "http://127.0.0.1:1234/v1",
		},
		Review: ReviewConfig{
			ChunkSize:    3000,
			MaxRetries:   3,
			ChunkTimeout: 15 * time.Minute,
			WriteTests:   false,
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

type FailedChunk struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}
type ReviewMode string

const (
	ModeDiffUncommitted ReviewMode = "diff-uncommitted"
	ModeDiffBranch      ReviewMode = "diff-branch"
	ModeReviewProject   ReviewMode = "review-project"
	ModeReviewFile      ReviewMode = "review-file"
)

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
type DiffProvider interface {
	GetUncommittedDiff(dir string) (string, error)
	GetBranchDiff(dir, base string) (string, error)
	ChunkDiff(diff string, chunkSize int) []string
	GetProjectChunks(dir string, chunkSize int, extensions []string) ([]string, error)
	GetFileChunks(path string, chunkSize int) ([]string, error)
}
type LLMProvider interface {
	ReviewChunk(ctx context.Context, prompt, code, lang string) (string, error)
	GenerateUnitTests(ctx context.Context, prompt, code, lang string) (string, error)
	HealthCheck(ctx context.Context) error
}
type ConfigProvider interface {
	Load(path string) (*Config, error)
}

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
