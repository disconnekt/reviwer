package review

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reviewer/pkg/types"
)

// Service provides review functionality
type Service struct {
	config       *types.Config
	diffProvider types.DiffProvider
	llmProvider  types.LLMProvider
}

// NewService creates a new review service
func NewService(config *types.Config, diffProvider types.DiffProvider, llmProvider types.LLMProvider) *Service {
	return &Service{
		config:       config,
		diffProvider: diffProvider,
		llmProvider:  llmProvider,
	}
}

// Review performs code review based on the provided options
func (s *Service) Review(ctx context.Context, opts *types.ReviewOptions) error {
	// Health check
	if err := s.llmProvider.HealthCheck(ctx); err != nil {
		return fmt.Errorf("LLM health check failed: %w", err)
	}

	var chunks []string
	var lang string
	var err error

	// Handle failed chunk resume mode
	if opts.ResumeFailed {
		chunks, lang, err = s.loadFailedChunks(opts)
		if err != nil {
			return fmt.Errorf("failed to load failed chunks: %w", err)
		}
	} else {
		chunks, lang, err = s.getChunksForMode(opts)
		if err != nil {
			return fmt.Errorf("failed to get chunks: %w", err)
		}
	}

	if len(chunks) == 0 {
		log.Println("No chunks to review")
		return nil
	}

	log.Printf("Starting review of %d chunks in language: %s", len(chunks), lang)

	// Get language configuration
	langCfg, exists := s.config.Languages[lang]
	if !exists {
		return fmt.Errorf("unsupported language: %s", lang)
	}

	// Perform review with retry logic
	return s.reviewWithRetry(ctx, chunks, lang, &langCfg, opts)
}

// getChunksForMode retrieves chunks based on the review mode
func (s *Service) getChunksForMode(opts *types.ReviewOptions) ([]string, string, error) {
	switch opts.Mode {
	case types.ModeDiffUncommitted:
		return s.getDiffChunks(opts.Dir, "", opts)
	case types.ModeDiffBranch:
		return s.getDiffChunks(opts.Dir, opts.Base, opts)
	case types.ModeReviewProject:
		return s.getProjectChunks(opts.Dir)
	case types.ModeReviewFile:
		return s.getFileChunks(opts.File)
	default:
		return nil, "", fmt.Errorf("unsupported review mode: %s", opts.Mode)
	}
}

// getDiffChunks retrieves and chunks diff content
func (s *Service) getDiffChunks(dir, base string, opts *types.ReviewOptions) ([]string, string, error) {
	var diff string
	var err error

	if base == "" {
		diff, err = s.diffProvider.GetUncommittedDiff(dir)
	} else {
		diff, err = s.diffProvider.GetBranchDiff(dir, base)
	}

	if err != nil {
		return nil, "", err
	}

	if len(diff) == 0 {
		return []string{}, "", nil
	}

	// Detect language from diff
	lang := s.detectLanguageFromDiff(diff)
	if lang == "" {
		// Default to first available language if detection fails
		for l := range s.config.Languages {
			lang = l
			break
		}
		if lang == "" {
			return nil, "", fmt.Errorf("no supported languages configured")
		}
	}

	chunks := s.diffProvider.ChunkDiff(diff, s.config.Review.ChunkSize)
	return chunks, lang, nil
}

// getProjectChunks retrieves chunks for entire project review
func (s *Service) getProjectChunks(dir string) ([]string, string, error) {
	// Find all supported languages in the project
	langFiles := make(map[string][]string)
	

	
	for lang, langCfg := range s.config.Languages {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip common directories that shouldn't be reviewed
				name := d.Name()
				if name == ".git" || name == "vendor" || name == "node_modules" || 
				   name == ".vscode" || name == ".idea" {
					return filepath.SkipDir
				}
				// Skip hidden directories except current directory
				if strings.HasPrefix(name, ".") && name != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, langCfg.Extension) {
				langFiles[lang] = append(langFiles[lang], path)
			}
			return nil
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to walk directory for %s files: %w", lang, err)
		}
	}

	// Choose the language with the most files
	var selectedLang string
	var maxFiles int
	for lang, files := range langFiles {
		if len(files) > maxFiles {
			maxFiles = len(files)
			selectedLang = lang
		}
	}

	if selectedLang == "" {
		return nil, "", fmt.Errorf("no supported language files found in project")
	}

	langCfg := s.config.Languages[selectedLang]
	chunks, err := s.diffProvider.GetProjectChunks(dir, s.config.Review.ChunkSize, []string{langCfg.Extension})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project chunks: %w", err)
	}

	return chunks, selectedLang, nil
}

// detectLanguageFromDiff detects programming language from diff content
func (s *Service) detectLanguageFromDiff(diff string) string {
	// Simple language detection based on file extensions in diff
	for lang, langCfg := range s.config.Languages {
		if strings.Contains(diff, langCfg.Extension) {
			return lang
		}
	}
	return ""
}

// retryWithBackoff implements exponential backoff retry logic
func (s *Service) retryWithBackoff(ctx context.Context, maxRetries int, operation func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := operation(); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				// Exponential backoff: 1s, 2s, 4s, etc.
				backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-time.After(backoffDuration):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return nil // Success
		}
	}
	return fmt.Errorf("operation failed after %d retries, last error: %w", maxRetries, lastErr)
}

// getFileChunks retrieves chunks for single file review
func (s *Service) getFileChunks(filePath string) ([]string, string, error) {
	// Detect language from filename
	diff, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}
	lang := s.detectLanguageFromDiff(string(diff))
	if lang == "" {
		supportedLangs := make([]string, 0, len(s.config.Languages))
		for l := range s.config.Languages {
			supportedLangs = append(supportedLangs, l)
		}
		return nil, "", fmt.Errorf("could not detect language from diff. Supported languages: %v", supportedLangs)
	}

	chunks, err := s.diffProvider.GetFileChunks(filePath, s.config.Review.ChunkSize)
	if err != nil {
		return nil, "", err
	}

	return chunks, lang, nil
}

// reviewWithRetry performs review with retry logic and progress tracking
func (s *Service) reviewWithRetry(ctx context.Context, chunks []string, lang string, langCfg *types.LanguageConfig, opts *types.ReviewOptions) error {
	var failedChunks []types.FailedChunk
	var mu sync.Mutex
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	
	defer func() {
		signal.Stop(sigChan)
		if len(failedChunks) > 0 {
			s.saveFailedChunks(failedChunks, opts.FailedChunksFile)
		}
	}()

	// Process chunks with timeout and retry
	for i, chunk := range chunks {
		select {
		case <-sigChan:
			log.Println("Received interrupt signal, saving progress...")
			return fmt.Errorf("interrupted by user")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("Processing chunk %d/%d", i+1, len(chunks))

		err := s.retryWithBackoff(ctx, opts.MaxRetries, func() error {
			chunkCtx, cancel := context.WithTimeout(ctx, opts.ChunkTimeout)
			defer cancel()

			result, err := s.llmProvider.ReviewChunk(chunkCtx, langCfg.ReviewPrompt, chunk, lang)
			if err != nil {
				return err
			}

			// Print review result
			fmt.Printf("\n=== Chunk %d Review ===\n%s\n\n", i+1, result)

			// Generate tests if requested
			if opts.WriteTests && langCfg.TestPrompt != "" {
				return s.generateAndRunTests(chunkCtx, chunk, lang, langCfg, opts, i)
			}

			return nil
		})

		if err != nil {
			log.Printf("Chunk %d failed after retries: %v", i+1, err)
			mu.Lock()
			failedChunks = append(failedChunks, types.FailedChunk{
				Index: i,
				Error: err.Error(),
			})
			mu.Unlock()
		}
	}

	if len(failedChunks) > 0 {
		log.Printf("Review completed with %d failed chunks", len(failedChunks))
		return fmt.Errorf("%d chunks failed processing", len(failedChunks))
	}

	log.Println("Review completed successfully")
	return nil
}

// generateAndRunTests generates and optionally runs tests for a code chunk
func (s *Service) generateAndRunTests(ctx context.Context, chunk, lang string, langCfg *types.LanguageConfig, opts *types.ReviewOptions, chunkIdx int) error {
	testGen, err := s.llmProvider.GenerateUnitTests(ctx, langCfg.TestPrompt, chunk, lang)
	if err != nil {
		return fmt.Errorf("failed to generate tests: %w", err)
	}

	testFiles, err := ParseAndWriteTests(testGen, lang, opts.Dir, chunkIdx)
	if err != nil {
		return fmt.Errorf("failed to write tests: %w", err)
	}

	if !opts.KeepTests {
		defer CleanupGeneratedTests(testFiles)
	}

	if err := RunTests(lang, opts.Dir); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	return nil
}

// loadFailedChunks loads previously failed chunks for retry
func (s *Service) loadFailedChunks(opts *types.ReviewOptions) ([]string, string, error) {
	f, err := os.Open(opts.FailedChunksFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open failed chunks file: %w", err)
	}
	defer f.Close()

	var failedChunks []types.FailedChunk
	if err := json.NewDecoder(f).Decode(&failedChunks); err != nil {
		return nil, "", fmt.Errorf("failed to decode failed chunks: %w", err)
	}

	// For simplicity, assume Go language and project mode
	// In a real implementation, this should be stored with the failed chunks
	lang := "go"
	langCfg := s.config.Languages[lang]
	
	allChunks, err := s.diffProvider.GetProjectChunks(opts.Dir, s.config.Review.ChunkSize, []string{langCfg.Extension})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project chunks: %w", err)
	}

	var chunks []string
	for _, fc := range failedChunks {
		if fc.Index >= 0 && fc.Index < len(allChunks) {
			chunks = append(chunks, allChunks[fc.Index])
		}
	}

	return chunks, lang, nil
}

// saveFailedChunks saves failed chunk information for later retry
func (s *Service) saveFailedChunks(failedChunks []types.FailedChunk, filename string) {
	if len(failedChunks) == 0 {
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create failed chunks file: %v", err)
		return
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(failedChunks); err != nil {
		log.Printf("Failed to encode failed chunks: %v", err)
	} else {
		log.Printf("Saved %d failed chunks to %s", len(failedChunks), filename)
	}
}

// ParseAndWriteTests parses generated test code and writes to files
func ParseAndWriteTests(testGen, lang, dir string, chunkIdx int) ([]string, error) {
	// This is a simplified implementation
	// In practice, you'd want more sophisticated parsing
	
	var testFiles []string
	
	// Extract code blocks from the generated content
	lines := strings.Split(testGen, "\n")
	var currentFile strings.Builder
	var fileName string
	inCodeBlock := false
	
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block, write file
				if fileName != "" && currentFile.Len() > 0 {
					fullPath := filepath.Join(dir, fileName)
					if err := os.WriteFile(fullPath, []byte(currentFile.String()), 0644); err != nil {
						return testFiles, fmt.Errorf("failed to write test file %s: %w", fullPath, err)
					}
					testFiles = append(testFiles, fullPath)
				}
				currentFile.Reset()
				fileName = ""
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
				// Try to extract filename from the line
				if strings.Contains(line, ".go") || strings.Contains(line, ".php") {
					parts := strings.Fields(line)
					for _, part := range parts {
						if strings.Contains(part, "."+lang) {
							fileName = part
							break
						}
					}
				}
				if fileName == "" {
					fileName = fmt.Sprintf("test_chunk_%d_test.%s", chunkIdx, getFileExtension(lang))
				}
			}
		} else if inCodeBlock {
			currentFile.WriteString(line + "\n")
		}
	}
	
	return testFiles, nil
}

// CleanupGeneratedTests removes generated test files
func CleanupGeneratedTests(files []string) {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			log.Printf("Failed to remove test file %s: %v", file, err)
		}
	}
}

// RunTests runs tests for the specified language
func RunTests(lang, dir string) error {
	var cmd *exec.Cmd
	
	switch lang {
	case "go":
		cmd = exec.Command("go", "test", "./...")
	case "php":
		cmd = exec.Command("phpunit", ".")
	default:
		return fmt.Errorf("unsupported language for testing: %s", lang)
	}
	
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return fmt.Errorf("tests failed: %w\nOutput: %s", err, string(output))
	}
	
	log.Printf("Tests passed:\n%s", string(output))
	return nil
}

// getFileExtension returns the file extension for a language
func getFileExtension(lang string) string {
	switch lang {
	case "go":
		return "go"
	case "php":
		return "php"
	default:
		return "txt"
	}
}
