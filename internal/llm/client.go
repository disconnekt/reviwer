package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"reviewer/pkg/types"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	provider     string
	model        string
	apiKey       string
	openaiClient *openai.Client
	httpClient   *http.Client
	lmstudioURL  string
}

func NewClient(llmCfg types.LLMConfig) *Client {
	client := &Client{
		provider:    llmCfg.Provider,
		model:       llmCfg.Model,
		apiKey:      llmCfg.APIKey,
		httpClient:  &http.Client{Timeout: 15 * time.Minute},
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

func (c *Client) HealthCheck(ctx context.Context) error {
	switch c.provider {
	case "lmstudio":
		return c.healthCheckLMStudio(ctx)
	case "openai":
		return c.healthCheckOpenAI(ctx)
	case "claude":
		return c.healthCheckClaude(ctx)
	case "mistral":
		return c.healthCheckMistral(ctx)
	case "groq":
		return c.healthCheckGroq(ctx)
	default:
		return fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

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

func (c *Client) healthCheckOpenAI(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}

	_, err := c.openaiClient.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("OpenAI health check failed: %w", err)
	}

	return nil
}

func (c *Client) ReviewChunk(ctx context.Context, prompt, code, lang string) (string, error) {
	switch c.provider {
	case "lmstudio":
		return c.lmstudioChat(ctx, prompt, code, lang)
	case "openai":
		return c.openaiChat(ctx, prompt, code, lang)
	case "claude":
		return c.claudeChat(ctx, prompt, code, lang)
	case "mistral":
		return c.mistralChat(ctx, prompt, code, lang)
	case "groq":
		return c.groqChat(ctx, prompt, code, lang)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

func (c *Client) GenerateUnitTests(ctx context.Context, prompt, code, lang string) (string, error) {
	switch c.provider {
	case "lmstudio":
		return c.lmstudioChatTest(ctx, prompt, code, lang)
	case "openai":
		return c.openaiChatTest(ctx, prompt, code, lang)
	case "claude":
		return c.claudeChatTest(ctx, prompt, code, lang)
	case "mistral":
		return c.mistralChatTest(ctx, prompt, code, lang)
	case "groq":
		return c.groqChatTest(ctx, prompt, code, lang)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

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

func (c *Client) lmstudioChat(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.lmstudioRequest(ctx, prompt, code, lang, 0.1, 2000)
}
func (c *Client) lmstudioChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.lmstudioRequest(ctx, prompt, code, lang, 0.2, 3000)
}

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

func RetryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i == maxRetries-1 {
				break
			}

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

func (c *Client) healthCheckClaude(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("Claude API key is required")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Claude health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 401 {
		return fmt.Errorf("Claude API key is invalid")
	}
	return nil
}

func (c *Client) healthCheckMistral(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("Mistral API key is required")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.mistral.ai/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Mistral health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 401 {
		return fmt.Errorf("Mistral API key is invalid")
	}
	return nil
}

func (c *Client) healthCheckGroq(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("Groq API key is required")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Groq health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 401 {
		return fmt.Errorf("Groq API key is invalid")
	}
	return nil
}

func (c *Client) claudeChat(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.claudeRequest(ctx, prompt, code, lang, 0.1, 2000)
}

func (c *Client) claudeChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.claudeRequest(ctx, prompt, code, lang, 0.2, 3000)
}

func (c *Client) claudeRequest(ctx context.Context, prompt, code, lang string, temperature float32, maxTokens int) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	reqBody := map[string]interface{}{
		"model":      c.model,
		"max_tokens": maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": fullPrompt},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API error %d: %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in Claude response")
	}
	
	return response.Content[0].Text, nil
}

func (c *Client) mistralChat(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.mistralRequest(ctx, prompt, code, lang, 0.1, 2000)
}

func (c *Client) mistralChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.mistralRequest(ctx, prompt, code, lang, 0.2, 3000)
}

func (c *Client) mistralRequest(ctx context.Context, prompt, code, lang string, temperature float32, maxTokens int) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	reqBody := map[string]interface{}{
		"model":       c.model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": fullPrompt},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.mistral.ai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Mistral request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Mistral API error %d: %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in Mistral response")
	}
	
	return response.Choices[0].Message.Content, nil
}

func (c *Client) groqChat(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.groqRequest(ctx, prompt, code, lang, 0.1, 2000)
}

func (c *Client) groqChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	return c.groqRequest(ctx, prompt, code, lang, 0.2, 3000)
}

func (c *Client) groqRequest(ctx context.Context, prompt, code, lang string, temperature float32, maxTokens int) (string, error) {
	fullPrompt := fmt.Sprintf("%s\n\nLanguage: %s\nCode:\n%s", prompt, lang, code)
	
	reqBody := map[string]interface{}{
		"model":       c.model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": fullPrompt},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Groq request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Groq API error %d: %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in Groq response")
	}
	
	return response.Choices[0].Message.Content, nil
}
