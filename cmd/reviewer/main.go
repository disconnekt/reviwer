package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"reviewer/internal/diff"
	"reviewer/internal/llm"
	"reviewer/internal/review"
	"reviewer/pkg/types"
)

func main() {
	var (
		mode         = flag.String("mode", "diff-uncommitted", "Review mode: diff-uncommitted, diff-branch, review-project, review-file")
		dir          = flag.String("dir", ".", "Directory to review")
		file         = flag.String("file", "", "File to review (for review-file mode)")
		base         = flag.String("base", "main", "Base branch for diff-branch mode")
		llmProvider  = flag.String("llm-provider", "openai", "LLM provider: openai, lmstudio, claude, mistral, groq")
		llmModel     = flag.String("llm-model", "gpt-4", "LLM model to use")
		llmBaseURL   = flag.String("llm-base-url", "http://127.0.0.1:1234/v1", "LLM base URL for LM Studio")
		writeTests   = flag.Bool("write-tests", false, "Generate unit tests")
		keepTests    = flag.Bool("keep-tests", false, "Keep generated test files")
		maxRetries   = flag.Int("max-retries", 3, "Maximum retry attempts for failed chunks")
		chunkTimeout = flag.Duration("chunk-timeout", 15*time.Minute, "Timeout for each chunk processing")
		resumeFailed = flag.Bool("resume-failed", false, "Resume processing from failed chunks")
	)
	flag.Parse()

	config := types.NewDefaultConfig()
	config.LLM.Provider = *llmProvider
	config.LLM.Model = *llmModel
	config.LLM.BaseURL = *llmBaseURL
	config.Review.WriteTests = *writeTests
	config.Review.KeepTests = *keepTests
	config.Review.MaxRetries = *maxRetries
	config.Review.ChunkTimeout = *chunkTimeout

	switch config.LLM.Provider {
	case "openai":
		if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
			config.LLM.APIKey = apiKey
		} else {
			log.Fatal("OPENAI_API_KEY environment variable is required when using OpenAI provider")
		}
	case "claude":
		if apiKey := os.Getenv("CLAUDE_API_KEY"); apiKey != "" {
			config.LLM.APIKey = apiKey
		} else {
			log.Fatal("CLAUDE_API_KEY environment variable is required when using Claude provider")
		}
	case "mistral":
		if apiKey := os.Getenv("MISTRAL_API_KEY"); apiKey != "" {
			config.LLM.APIKey = apiKey
		} else {
			log.Fatal("MISTRAL_API_KEY environment variable is required when using Mistral provider")
		}
	case "groq":
		if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
			config.LLM.APIKey = apiKey
		} else {
			log.Fatal("GROQ_API_KEY environment variable is required when using Groq provider")
		}
	}

	var reviewMode types.ReviewMode
	switch *mode {
	case "diff-uncommitted":
		reviewMode = types.ModeDiffUncommitted
	case "diff-branch":
		reviewMode = types.ModeDiffBranch
	case "review-project":
		reviewMode = types.ModeReviewProject
	case "review-file":
		reviewMode = types.ModeReviewFile
	default:
		log.Fatalf("Invalid mode: %s. Valid modes: diff-uncommitted, diff-branch, review-project, review-file", *mode)
	}

	options := &types.ReviewOptions{
		Mode:             reviewMode,
		Dir:              *dir,
		File:             *file,
		Base:             *base,
		WriteTests:       *writeTests,
		KeepTests:        *keepTests,
		MaxRetries:       *maxRetries,
		ChunkTimeout:     *chunkTimeout,
		FailedChunksFile: "failed_chunks.json",
		ResumeFailed:     *resumeFailed,
	}

	diffProvider := diff.NewProvider()
	llmClient := llm.NewClient(config.LLM)

	reviewService := review.NewService(config, diffProvider, llmClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reviewService.Review(ctx, options); err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	fmt.Println("Review completed successfully!")
}
