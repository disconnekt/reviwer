package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"reviewer/pkg/types"
)

// Client handles LLM operations with different providers
type Client struct {
	provider     string
	model        string
	apiKey       string
	openaiClient *openai.Client
	httpClient   *http.Client
	lmstudioURL  string
}

// NewClient creates a new LLM client based on configuration
func NewClient(llmCfg types.LLMConfig) *Client {
	client := &Client{
		provider:    llmCfg.Provider,
		model:       llmCfg.Model,
		apiKey:      llmCfg.APIKey,
		httpClient:  &http.Client{Timeout: 15 * time.Minute}, // Увеличен таймаут для LM Studio
		lmstudioURL: "http://127.0.0.1:1234/v1/chat/completions",
	}

	if llmCfg.BaseURL != "" {
		client.lmstudioURL = llmCfg.BaseURL + "/chat/completions"
	}

	if llmCfg.Provider == "openai" {
		client.openaiClient = openai.NewClient(llmCfg.APIKey)
	}

	return client
}

// HealthCheck verifies that the LLM backend is accessible
func (c *Client) HealthCheck(ctx context.Context) error {
	switch c.provider {
	case "lmstudio":
		return c.healthCheckLMStudio(ctx)
	case "openai":
		return c.healthCheckOpenAI(ctx)
	default:
		return fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

// healthCheckLMStudio checks LM Studio availability
func (c *Client) healthCheckLMStudio(ctx context.Context) error {
	modelsURL := strings.Replace(c.lmstudioURL, "/v1/chat/completions", "/v1/models", 1)
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("lmstudio health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lmstudio health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// healthCheckOpenAI checks OpenAI API availability
func (c *Client) healthCheckOpenAI(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}

	// Try to list models as a simple health check
	_, err := c.openaiClient.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("OpenAI health check failed: %w", err)
	}

	return nil
}

// ReviewChunk sends code chunk for review to the LLM
func (c *Client) ReviewChunk(ctx context.Context, prompt, code, lang string) (string, error) {
	switch c.provider {
	case "lmstudio":
		return c.lmstudioChat(ctx, prompt, code, lang)
	case "openai":
		return c.openaiChat(ctx, prompt, code, lang)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

// GenerateUnitTests generates unit tests for code chunk
func (c *Client) GenerateUnitTests(ctx context.Context, prompt, code, lang string) (string, error) {
	switch c.provider {
	case "lmstudio":
		return c.lmstudioChatTest(ctx, prompt, code, lang)
	case "openai":
		return c.openaiChatTest(ctx, prompt, code, lang)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

// openaiChat handles OpenAI chat completion for code review
func (c *Client) openaiChat(ctx context.Context, prompt, code, lang string) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	resp, err := c.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fullPrompt,
			},
		},
		Temperature: 0.1,
		MaxTokens:   2000,
	})

	if err != nil {
		return "", fmt.Errorf("OpenAI chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// openaiChatTest handles OpenAI chat completion for test generation
func (c *Client) openaiChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	resp, err := c.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fullPrompt,
			},
		},
		Temperature: 0.2,
		MaxTokens:   3000,
	})

	if err != nil {
		return "", fmt.Errorf("OpenAI test generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// lmstudioChat handles LM Studio chat completion for code review
func (c *Client) lmstudioChat(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.lmstudioRequest(ctx, prompt, code, lang, 0.1, 2000)
}

// lmstudioChatTest handles LM Studio chat completion for test generation
func (c *Client) lmstudioChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.lmstudioRequest(ctx, prompt, code, lang, 0.2, 3000)
}

// lmstudioRequest makes a request to LM Studio
func (c *Client) lmstudioRequest(ctx context.Context, prompt, code, lang string, temperature float32, maxTokens int) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": fullPrompt,
			},
		},
		"temperature": temperature,
		"max_tokens":  maxTokens,
		"stream":      false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.lmstudioURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LM Studio request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode LM Studio response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LM Studio")
	}

	return response.Choices[0].Message.Content, nil
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i == maxRetries-1 {
				break
			}
			
			// Exponential backoff: 1s, 2s, 4s, 8s, ...
			backoff := time.Duration(1<<uint(i)) * time.Second
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		} else {
			return nil
		}
	}
	
	return fmt.Errorf("operation failed after %d retries, last error: %w", maxRetries, lastErr)
}
