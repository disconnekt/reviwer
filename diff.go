package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetUncommittedDiff(dir string) (string, error) {
	cmd := exec.Command("git", "diff", "--unified=3", dir)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetBranchDiff(dir, base string) (string, error) {
	cmd := exec.Command("git", "diff", base+"...HEAD", "--unified=3", dir)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetProjectChunks(dir string, chunkSize int, extensions []string) ([]string, error) {
	var chunks []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		for _, ext := range extensions {
			if strings.HasSuffix(path, ext) {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				lines := strings.Split(string(data), "\n")
				for i := 0; i < len(lines); i += chunkSize {
					end := i + chunkSize
					if end > len(lines) {
						end = len(lines)
					}
					chunk := strings.Join(lines[i:end], "\n")
					chunks = append(chunks, chunk)
				}
			}
		}
		return nil
	})
	return chunks, err
}

func GetFileChunks(path string, chunkSize int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var chunks []string
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunk := strings.Join(lines[i:end], "\n")
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func ChunkDiff(diff string, chunkSize int) []string {
	lines := strings.Split(diff, "\n")
	var chunks []string
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunk := strings.Join(lines[i:end], "\n")
		chunks = append(chunks, chunk)
	}
	return chunks
}
