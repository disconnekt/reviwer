package diff

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) GetUncommittedDiff(dir string) (string, error) {
	cmd := exec.Command("git", "diff", "--unified=3", dir)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed for %s: %w (stderr: %s)", dir, err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute git diff for %s: %w", dir, err)
	}
	return string(out), nil
}

func (p *Provider) GetBranchDiff(dir, base string) (string, error) {
	cmd := exec.Command("git", "diff", base+"...HEAD", "--unified=3", dir)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed for %s (base: %s): %w (stderr: %s)", dir, base, err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute git diff for %s (base: %s): %w", dir, base, err)
	}
	return string(out), nil
}

func (p *Provider) ChunkDiff(diff string, chunkSize int) []string {
	if diff == "" {
		return []string{}
	}

	lines := strings.Split(diff, "\n")
	if len(lines) == 0 {
		return []string{}
	}

	// Pre-allocate with estimated capacity
	chunks := make([]string, 0, (len(lines)+chunkSize-1)/chunkSize)

	var builder strings.Builder
	for i := 0; i < len(lines); i += chunkSize {
		builder.Reset()
		end := min(i+chunkSize, len(lines))

		for j := i; j < end; j++ {
			if j > i {
				builder.WriteByte('\n')
			}
			builder.WriteString(lines[j])
		}

		chunk := builder.String()
		if strings.TrimSpace(chunk) != "" { // Skip empty chunks
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

func (p *Provider) GetProjectChunks(dir string, chunkSize int, extensions []string) ([]string, error) {
	if len(extensions) == 0 {
		return nil, fmt.Errorf("no extensions specified")
	}

	var chunks []string
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[ext] = true
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory %s: %w", path, err)
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" ||
				name == ".vscode" || name == ".idea" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		hasTargetExt := false
		for ext := range extMap {
			if strings.HasSuffix(path, ext) {
				hasTargetExt = true
				break
			}
		}

		if !hasTargetExt {
			return nil
		}

		fileChunks, err := p.GetFileChunks(path, chunkSize)
		if err != nil {
			return fmt.Errorf("failed to chunk file %s: %w", path, err)
		}

		for i, chunk := range fileChunks {
			contextualChunk := fmt.Sprintf("// File: %s (chunk %d/%d)\n%s",
				path, i+1, len(fileChunks), chunk)
			chunks = append(chunks, contextualChunk)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk project directory %s: %w", dir, err)
	}

	return chunks, nil
}

func (p *Provider) GetFileChunks(path string, chunkSize int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	content := string(data)
	if content == "" {
		return []string{}, nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return []string{}, nil
	}

	chunks := make([]string, 0, (len(lines)+chunkSize-1)/chunkSize)

	var builder strings.Builder
	for i := 0; i < len(lines); i += chunkSize {
		builder.Reset()
		end := min(i+chunkSize, len(lines))

		for j := i; j < end; j++ {
			if j > i {
				builder.WriteByte('\n')
			}
			builder.WriteString(lines[j])
		}

		chunk := builder.String()
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
