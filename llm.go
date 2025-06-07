package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const defaultMaxSize = 20000

type FailedChunk struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type LLMClient struct {
	provider     string
	model        string
	apiKey       string
	openaiClient *openai.Client
	httpClient   *http.Client
	lmstudioURL  string
}

// HealthCheck checks if the LLM backend is reachable.
func (l *LLMClient) HealthCheck(ctx context.Context) error {
	if l.provider == "lmstudio" {
		req, err := http.NewRequestWithContext(ctx, "GET", strings.Replace(l.lmstudioURL, "/v1/chat/completions", "/v1/models", 1), nil)
		if err != nil {
			return err
		}
		resp, err := l.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("lmstudio health check failed, status: %d", resp.StatusCode)
		}
		return nil
	}
	// For OpenAI, assume always reachable if apiKey is set
	if l.provider == "openai" && l.apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is not set")
	}
	return nil
}

// retryWithBackoff retries a function up to maxRetries with exponential backoff.
func retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var err error
	backoff := time.Second
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return err
}


func NewLLMClientWithProvider(cfg *Config, apiKey string) *LLMClient {
	if cfg.LLMProvider == "lmstudio" {
		return &LLMClient{
			provider:    "lmstudio",
			model:       cfg.LLMModel,
			apiKey:      apiKey,
			httpClient:  &http.Client{},
			lmstudioURL: "http://127.0.0.1:1234/v1/chat/completions",
		}
	}
	return &LLMClient{
		provider:     "openai",
		model:        cfg.Model,
		apiKey:       apiKey,
		openaiClient: openai.NewClient(apiKey),
	}
}

func (l *LLMClient) ReviewChunk(ctx context.Context, prompt, code, lang string) (string, error) {
	if l.provider == "openai" {
		resp, err := l.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: l.model,
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt,
			}, {
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Here is the %s code diff chunk to review:\n\n```%s\n%s\n```", lang, lang, code),
			}},
			MaxTokens: 2048,
		})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "No response from LLM", nil
		}
		return resp.Choices[0].Message.Content, nil
	}

	return l.lmstudioChat(ctx, prompt, code, lang)
}

func (l *LLMClient) lmstudioChat(ctx context.Context, prompt, code, lang string) (string, error) {
	var result string
	var lastErr error
	err := retryWithBackoff(ctx, 3, func() error {
		type msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		body := map[string]interface{}{
			"model": l.model,
			"messages": []msg{
				{Role: "system", Content: prompt},
				{Role: "user", Content: fmt.Sprintf("Here is the %s code diff chunk to review:\n\n```%s\n%s\n```", lang, lang, code)},
			},
			"max_tokens": 2048,
		}
		b, err := json.Marshal(body)
		if err != nil {
			lastErr = err
			return err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", l.lmstudioURL, bytes.NewReader(b))
		if err != nil {
			lastErr = err
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := l.httpClient.Do(req)
		if err != nil {
			lastErr = err
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("lmstudio status %d", resp.StatusCode)
			return lastErr
		}
		var respBody struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			lastErr = err
			return err
		}
		if len(respBody.Choices) == 0 {
			lastErr = fmt.Errorf("No response from LLM")
			return lastErr
		}
		result = respBody.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", lastErr
	}
	return result, nil
}

func (l *LLMClient) GenerateUnitTests(ctx context.Context, prompt, code, lang string) (string, error) {
	if l.provider == "openai" {
		resp, err := l.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: l.model,
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt,
			}, {
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Generate unit tests for this %s code diff:\n\n```%s\n%s\n```", lang, lang, code),
			}},
			MaxTokens: 2048,
		})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "No response from LLM", nil
		}
		return resp.Choices[0].Message.Content, nil
	}
	// LM Studio
	return l.lmstudioChatTest(ctx, prompt, code, lang)
}

func (l *LLMClient) lmstudioChatTest(ctx context.Context, prompt, code, lang string) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := map[string]interface{}{
		"model": l.model,
		"messages": []msg{
			{Role: "system", Content: prompt},
			{Role: "user", Content: fmt.Sprintf("Generate unit tests for this %s code diff:\n\n```%s\n%s\n```", lang, lang, code)},
		},
		"max_tokens": 2048,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", l.lmstudioURL, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("lmstudio status %d", resp.StatusCode)
	}
	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", err
	}
	if len(respBody.Choices) == 0 {
		return "No response from LLM", nil
	}
	return respBody.Choices[0].Message.Content, nil
}

func ParseAndWriteTests(testGen, lang, dir string, chunkIdx int) ([]string, error) {
	var files []string
	start := 0
	blockNum := 0
	for {
		codeStart := strings.Index(testGen[start:], "```")
		if codeStart == -1 {
			break
		}
		codeStart += start + 3
		codeEnd := strings.Index(testGen[codeStart:], "```")
		if codeEnd == -1 {
			break
		}
		codeEnd += codeStart
		codeBlock := testGen[codeStart:codeEnd]
		var filename string
		switch lang {
		case "go":
			filename = filepath.Join(dir, fmt.Sprintf("llm_generated_test_%d_%d_%d.go", chunkIdx, blockNum, time.Now().UnixNano()))
		case "php":
			filename = filepath.Join(dir, fmt.Sprintf("LLMGeneratedTest_%d_%d_%d.php", chunkIdx, blockNum, time.Now().UnixNano()))
		default:
			return files, fmt.Errorf("test writing not supported for language: %s", lang)
		}
		err := os.WriteFile(filename, []byte(codeBlock), 0644)
		if err != nil {
			return files, err
		}
		files = append(files, filename)
		blockNum++
		start = codeEnd + 3
	}
	if len(files) == 0 {
		return files, fmt.Errorf("no code blocks found in LLM test output")
	}
	return files, nil
}

func CleanupGeneratedTests(files []string) {
	for _, f := range files {
		_ = os.Remove(f)
	}
}

func RunTests(lang, dir string) error {
	var cmd *exec.Cmd
	switch lang {
	case "go":
		cmd = exec.Command("go", "test", "./...")
		cmd.Dir = dir
	case "php":
		cmd = exec.Command("phpunit")
		cmd.Dir = dir
	default:
		return fmt.Errorf("test running not supported for language: %s", lang)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (l *LLMClient) ReviewAndFixLoop(ctx context.Context, cfg *Config, lang string, chunks []string, writeTests bool, dir string, keepTests bool, chunkTimeout time.Duration, maxRetries int, failedChunksFile string) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[!] Panic in review loop: %v\n", r)
		}
	}()
	// Handle SIGINT for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	interrupted := false
	go func() {
		<-sigChan
		fmt.Println("\n[!] Interrupted by user (SIGINT). Printing summary...")
		interrupted = true
	}()
	var langCfg *LanguageConfig
	switch lang {
	case "go":
		v := cfg.Languages["go"]
		langCfg = &v
	case "php":
		v := cfg.Languages["php"]
		langCfg = &v
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}
	reviewPrompt := langCfg.ReviewPrompt
	testPrompt := langCfg.TestPrompt
	var (
		totalTests   int
		testsPassed  int
		testsFailed  int
		writtenFiles []string
	)

	var failedChunks []FailedChunk
	timeoutCount := 0
	for i, chunk := range chunks {
		if interrupted {
			break
		}
		fmt.Printf("\n--- Reviewing chunk %d/%d [%s] ---\n", i+1, len(chunks), lang)
		fmt.Fprintf(os.Stderr, "[DEBUG] Starting review for chunk %d/%d\n", i+1, len(chunks))
		retries := 0
		var review string
		var err error
		for retries = 0; retries < maxRetries; retries++ {
			chunkCtx, cancel := context.WithTimeout(ctx, chunkTimeout)
			review, err = l.ReviewChunk(chunkCtx, reviewPrompt, chunk, lang)
			cancel()
			if err == nil {
				break
			}
			fmt.Fprintf(os.Stderr, "[!] Review error in chunk %d (attempt %d/%d): %v\n", i+1, retries+1, maxRetries, err)
			fmt.Fprintf(os.Stderr, "[DEBUG] Review returned error: %v\n", err)
			fmt.Fprintf(os.Stderr, "[DEBUG] Review content: %q\n", review)
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					timeoutCount++
					if timeoutCount >= 3 {
						fmt.Fprintf(os.Stderr, "[!] Warning: %d consecutive chunk timeouts. Check your LLM backend or consider increasing --chunk-timeout.\n", timeoutCount)
					}
				} else {
					timeoutCount = 0
				}
				failedChunks = append(failedChunks, FailedChunk{Index: i, Error: err.Error()})
				continue
			} else {
				timeoutCount = 0
			}
		}
		fmt.Fprintf(os.Stderr, "[DEBUG] Review for chunk %d: %q\n", i+1, review)
		if review == "" {
			fmt.Fprintf(os.Stderr, "[WARNING] LLM returned an empty review for chunk %d.\n", i+1)
		}

		retries = 0
		var testGen string
		for retries = 0; retries < maxRetries; retries++ {
			chunkCtx, cancel := context.WithTimeout(ctx, chunkTimeout)
			testGen, err = l.GenerateUnitTests(chunkCtx, testPrompt, chunk, lang)
			cancel()
			if err == nil {
				break
			}
			fmt.Fprintf(os.Stderr, "[!] Test generation error in chunk %d (attempt %d/%d): %v\n", i+1, retries+1, maxRetries, err)
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				timeoutCount++
				if timeoutCount >= 3 {
					fmt.Fprintf(os.Stderr, "[!] Warning: %d consecutive chunk timeouts. Check your LLM backend or consider increasing --chunk-timeout.\n", timeoutCount)
				}
			} else {
				timeoutCount = 0
			}
			fmt.Fprintf(os.Stderr, "[!] Test generation error in chunk %d: %v\n", i+1, err)
			continue
		}
		fmt.Println("\nUnit test suggestions/generation:\n", testGen)

		if writeTests {
			timeoutCount = 0 // Reset on successful chunk
			files, err := ParseAndWriteTests(testGen, lang, dir, i)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] Failed to write tests: %v\n", err)
			} else {
				fmt.Printf("[+] Wrote generated tests: %v\n", files)
				writtenFiles = append(writtenFiles, files...)
				totalTests += len(files)
				err = RunTests(lang, dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[!] Test run failed: %v\n", err)
					testsFailed += len(files)
				} else {
					fmt.Println("[+] Tests passed.")
					testsPassed += len(files)
				}
			}
		}
		fmt.Printf("[Chunk %d] Done.\n", i+1)
	}

	// Print summary and cleanup after all chunks processed
	fmt.Printf("\n===== SUMMARY for %s =====\n", lang)
	if writeTests && !keepTests {
		CleanupGeneratedTests(writtenFiles)
		log.Println("[+] Cleaned up generated test files.")
	}
	if len(failedChunks) > 0 && failedChunksFile != "" {
		f, err := os.Create(failedChunksFile)
		if err != nil {
			log.Printf("[!] Could not write failed chunks file %s: %v\n", failedChunksFile, err)
		} else {
			if err := json.NewEncoder(f).Encode(failedChunks); err != nil {
				log.Printf("[!] Failed to encode failed chunks: %v\n", err)
			}
			if err := f.Close(); err != nil {
				log.Printf("[!] Failed to close failed chunks file: %v\n", err)
			}
			log.Printf("[!] Wrote failed chunks to %s. Use --resume-failed to retry only failed chunks.\n", failedChunksFile)
		}
	}
	return nil
}
