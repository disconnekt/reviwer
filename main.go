package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)
func main() {
	maxRetries := flag.Int("max-retries", 1, "Max retries for failed chunks")
	failedChunksFile := flag.String("failed-chunks-file", "failed_chunks.json", "File to save/read failed chunk indices")
	resumeFailed := flag.Bool("resume-failed", false, "Only process failed chunks from failed-chunks-file")
	// Add chunk-timeout flag (default 5m)
	chunkTimeout := flag.Duration("chunk-timeout", 5*time.Minute, "Timeout for each review chunk (e.g. 2m, 30s)")
	apiKey := os.Getenv("OPENAI_API_KEY")
	configPath := flag.String("config", "config.toml", "Path to config.toml")
	mode := flag.String("mode", "diff-uncommitted", "Mode: diff-uncommitted, diff-branch, review-project, review-file")
	dir := flag.String("dir", ".", "Project directory for diff or review")
	file := flag.String("file", "", "Single file to review")
	base := flag.String("base", "master", "Base branch for diff-branch mode")
	writeTests := flag.Bool("write-tests", false, "Automatically write and run generated tests")
	keepTests := flag.Bool("keep-tests", false, "Keep generated test files after run (default: false)")
	llmProvider := flag.String("llm-provider", "", "LLM provider: openai or lmstudio (overrides config)")
	llmModel := flag.String("llm-model", "", "LLM model name for LM Studio or OpenAI (overrides config)")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var chunks []string
	var lang string
	if *resumeFailed {
		// Read failed chunks indices from file
		f, err := os.Open(*failedChunksFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open failed chunks file: %v\n", err)
			os.Exit(1)
		}
		var failedChunks []struct{ Index int }
		err = json.NewDecoder(f).Decode(&failedChunks)
		if err := f.Close(); err != nil {
			log.Printf("error closing file: %v", err)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decode failed chunks file: %v\n", err)
			os.Exit(1)
		}
		// For simplicity, assume review-project mode and Go for now
		lang = "go"
		// Load all project chunks
		ext := cfg.Languages[lang].Extension
		allChunks, _ := GetProjectChunks(*dir, cfg.ChunkSize, []string{ext})
		for _, fc := range failedChunks {
			if fc.Index >= 0 && fc.Index < len(allChunks) {
				chunks = append(chunks, allChunks[fc.Index])

			}
		}
	} // else normal mode logic below

	if *llmProvider != "" {
		cfg.LLMProvider = *llmProvider
	}
	if *llmModel != "" {
		cfg.LLMModel = *llmModel
	}
	if cfg.LLMProvider == "lmstudio" {
		allowed := map[string]bool{"claude-3.7-sonnet-reasoning-gemma3-12b": true, "google/gemma-3-12b": true, "openchat_3.5": true}
		if !allowed[cfg.LLMModel] {
			fmt.Fprintf(os.Stderr, "Invalid lmstudio model: %s\n", cfg.LLMModel)
			os.Exit(1)
		}
	}
	fmt.Printf("[LLM] Provider: %s | Model: %s\n", cfg.LLMProvider, cfg.LLMModel)

	llm := NewLLMClientWithProvider(cfg, apiKey)
	ctx := context.Background()
	if err := llm.HealthCheck(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[!] LLM backend health check failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please ensure the LLM backend is running and accessible. For lmstudio, check http://127.0.0.1:1234/v1/models in your browser.")
		os.Exit(1)
	}

	getLangConfig := func(lang string) *LanguageConfig {
		switch lang {
		case "go":
			v := cfg.Languages["go"]
			return &v
		case "php":
			v := cfg.Languages["php"]
			return &v
		default:
			return nil
		}
	}

	switch *mode {
	case "diff-uncommitted":
		diff, err := GetUncommittedDiff(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get uncommitted diff: %v\n", err)
			os.Exit(1)
		}
		if strings.Contains(diff, getLangConfig("go").Extension) {
			lang = "go"
		} else if strings.Contains(diff, getLangConfig("php").Extension) {
			lang = "php"
		} else {
			lang = "go"
		}
		chunks = ChunkDiff(diff, cfg.ChunkSize)
		if lang == "" {
			fmt.Fprintln(os.Stderr, "Could not detect language from diff. Supported: ", "go", "php")
			os.Exit(1)
		}
	case "diff-branch":
		diff, err := GetBranchDiff(*dir, *base)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get branch diff: %v\n", err)
			os.Exit(1)
		}
		if len(diff) == 0 {
			fmt.Println("No branch diff changes detected.")
			return
		}
		lang = detectLangFromDiff(diff, cfg)
		if lang == "" {
			fmt.Fprintln(os.Stderr, "Could not detect language from diff. Supported: ", keys(cfg.Languages))
			os.Exit(1)
		}
		chunks = ChunkDiff(diff, cfg.ChunkSize)
	case "review-project":
		langFiles := map[string][]string{"go": {}, "php": {}}
		extByLang := map[string]string{"go": cfg.Languages["go"].Extension, "php": cfg.Languages["php"].Extension}
		err = filepath.WalkDir(*dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			for l, ext := range extByLang {
				if strings.HasSuffix(path, ext) {
					langFiles[l] = append(langFiles[l], path)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to scan project files: %v\n", err)
			os.Exit(1)
		}
		if len(langFiles["go"]) == 0 && len(langFiles["php"]) == 0 {
			fmt.Fprintln(os.Stderr, "No supported files found in project.")
			os.Exit(1)
		}
		for l, files := range langFiles {
			if len(files) == 0 {
				continue
			}
			fmt.Printf("\n===== Reviewing language: %s (%d files) =====\n", l, len(files))
			var langChunks []string
			for _, f := range files {
				chunks, err := GetFileChunks(f, cfg.ChunkSize)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[!] Failed to chunk file %s: %v\n", f, err)
					continue
				}
				langChunks = append(langChunks, chunks...)
			}
			if len(langChunks) == 0 {
				fmt.Fprintf(os.Stderr, "[!] No chunks to review for language %s\n", l)
				continue
			}
			err = llm.ReviewAndFixLoop(ctx, cfg, l, langChunks, *writeTests, *dir, *keepTests, *chunkTimeout, *maxRetries, *failedChunksFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] Review/fix loop failed for %s: %v\n", l, err)
			}
		}
		return
	case "review-file":
		if *file == "" {
			fmt.Fprintln(os.Stderr, "--file must be specified for review-file mode")
			os.Exit(1)
		}
		chunks, err = GetFileChunks(*file, cfg.ChunkSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get file chunks: %v\n", err)
			os.Exit(1)
		}
		lang = detectLangFromFilename(*file, cfg)
		if lang == "" {
			fmt.Fprintln(os.Stderr, "Could not detect language from file. Supported: ", keys(cfg.Languages))
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "Unknown mode. Use one of: diff-uncommitted, diff-branch, review-project, review-file")
		os.Exit(1)
	}

	err = llm.ReviewAndFixLoop(ctx, cfg, lang, chunks, *writeTests, *dir, *keepTests, *chunkTimeout, *maxRetries, *failedChunksFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Review failed: %v\n", err)
		os.Exit(1)
	}
}

func detectLangFromDiff(diff string, cfg *Config) string {
	for l, lcfg := range cfg.Languages {
		if strings.Contains(diff, lcfg.Extension) {
			return l
		}
	}
	return ""
}

func detectLangFromFilename(filename string, cfg *Config) string {
	for l, lcfg := range cfg.Languages {
		if strings.HasSuffix(filename, lcfg.Extension) {
			return l
		}
	}
	return ""
}

func keys(m map[string]LanguageConfig) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
