package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLI_ReviewFile_NoAPIKey(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--mode", "review-file", "--file", "main.go")
	cmd.Env = append(os.Environ(), "OPENAI_API_KEY=") // Unset key
	output, err := cmd.CombinedOutput()
	msg := string(output)
	if err == nil || (!strings.Contains(msg, "OPENAI_API_KEY is not set") && !strings.Contains(msg, "OPENAI_API_KEY environment variable is required") && !strings.Contains(msg, "You didn't provide an API key")) {
		t.Errorf("Expected error for missing API key, got: %s", output)
	}
}

func TestCLI_ReviewFile_Help(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Help command failed: %v", err)
	}
	if !strings.Contains(string(output), "-mode") && !strings.Contains(string(output), "--mode") {
		t.Errorf("Help output missing mode flag: %s", output)
	}
}

