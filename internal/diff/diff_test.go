package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvider_ChunkDiff(t *testing.T) {
	provider := NewProvider()
	
	tests := []struct {
		name      string
		diff      string
		chunkSize int
		expected  int // number of chunks
	}{
		{
			name:      "empty diff",
			diff:      "",
			chunkSize: 10,
			expected:  0,
		},
		{
			name:      "single line",
			diff:      "line1",
			chunkSize: 10,
			expected:  1,
		},
		{
			name:      "multiple lines, single chunk",
			diff:      "line1\nline2\nline3",
			chunkSize: 10,
			expected:  1,
		},
		{
			name:      "multiple lines, multiple chunks",
			diff:      "line1\nline2\nline3\nline4\nline5",
			chunkSize: 2,
			expected:  3,
		},
		{
			name:      "whitespace only",
			diff:      "   \n   \n   ",
			chunkSize: 10,
			expected:  0, // Should skip empty chunks
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := provider.ChunkDiff(tt.diff, tt.chunkSize)
			if len(chunks) != tt.expected {
				t.Errorf("Expected %d chunks, got %d", tt.expected, len(chunks))
			}
		})
	}
}

func TestProvider_GetFileChunks(t *testing.T) {
	provider := NewProvider()
	
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func helper() {
	// This is a helper function
	return
}`
	
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	tests := []struct {
		name      string
		chunkSize int
		expected  int // number of chunks
	}{
		{
			name:      "small chunks",
			chunkSize: 3,
			expected:  4, // 12 lines / 3 = 4 chunks
		},
		{
			name:      "large chunks",
			chunkSize: 20,
			expected:  1, // All lines fit in one chunk
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := provider.GetFileChunks(testFile, tt.chunkSize)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(chunks) != tt.expected {
				t.Errorf("Expected %d chunks, got %d", tt.expected, len(chunks))
			}
		})
	}
}

func TestProvider_GetFileChunks_NonExistentFile(t *testing.T) {
	provider := NewProvider()
	
	_, err := provider.GetFileChunks("nonexistent.go", 10)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestProvider_GetProjectChunks(t *testing.T) {
	provider := NewProvider()
	
	// Create a temporary project structure
	tmpDir := t.TempDir()
	
	// Create some Go files
	goFile1 := filepath.Join(tmpDir, "main.go")
	goFile2 := filepath.Join(tmpDir, "helper.go")
	phpFile := filepath.Join(tmpDir, "index.php")
	txtFile := filepath.Join(tmpDir, "readme.txt")
	
	files := map[string]string{
		goFile1: "package main\n\nfunc main() {\n\t// Main function\n}",
		goFile2: "package main\n\nfunc helper() {\n\t// Helper function\n}",
		phpFile: "<?php\necho 'Hello World';\n?>",
		txtFile: "This is a readme file",
	}
	
	for file, content := range files {
		err := os.WriteFile(file, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}
	
	// Create a subdirectory with a Go file
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	subGoFile := filepath.Join(subDir, "sub.go")
	err = os.WriteFile(subGoFile, []byte("package subdir\n\nfunc SubFunc() {}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write sub Go file: %v", err)
	}
	
	tests := []struct {
		name       string
		extensions []string
		minChunks  int // minimum expected chunks
	}{
		{
			name:       "go files only",
			extensions: []string{".go"},
			minChunks:  3, // 3 Go files
		},
		{
			name:       "php files only",
			extensions: []string{".php"},
			minChunks:  1, // 1 PHP file
		},
		{
			name:       "multiple extensions",
			extensions: []string{".go", ".php"},
			minChunks:  4, // 3 Go + 1 PHP
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := provider.GetProjectChunks(tmpDir, 10, tt.extensions)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(chunks) < tt.minChunks {
				t.Errorf("Expected at least %d chunks, got %d", tt.minChunks, len(chunks))
			}
			
			// Verify chunks contain file paths
			for _, chunk := range chunks {
				if !strings.Contains(chunk, "// File:") {
					t.Error("Chunk should contain file path context")
				}
			}
		})
	}
}

func TestProvider_GetProjectChunks_EmptyExtensions(t *testing.T) {
	provider := NewProvider()
	
	_, err := provider.GetProjectChunks(".", 10, []string{})
	if err == nil {
		t.Error("Expected error for empty extensions")
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{10, 10, 10},
		{0, 1, 0},
	}
	
	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
