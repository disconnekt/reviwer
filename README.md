# Reviewer: LLM-Powered Code Review and Test Suggestion Tool

A comprehensive Go-based code review tool that leverages Large Language Models (LLMs) to provide intelligent code analysis, suggestions, and automated test generation. Supports multiple LLM providers including OpenAI and LM Studio.

## Features
- **Multi-Provider LLM Support**: Works with OpenAI GPT models and LM Studio
- **Multiple Review Modes**: 
  - Diff review (uncommitted changes)
  - Branch comparison
  - Full project analysis
  - Single file review
- **Language Support**: Extensible support for Go, PHP, and other languages
- **Test Generation**: Automatic unit test creation and execution
- **Chunking Strategy**: Intelligent code chunking for large codebases
- **Retry Logic**: Robust error handling with exponential backoff
- **Progress Tracking**: Resume failed reviews from where they left off
- **Configurable**: TOML-based configuration with validation
- Automated code review using LLMs (OpenAI or LM Studio)
- Batch processing of large codebases (chunked review)
- Unit test generation and suggestion per code chunk
- Unique test file naming to avoid overwrites
- Robust error handling and retry logic
- Detailed logging and summary output
- Automatic cleanup of generated test files (optional)
- CLI integration test for reliability

## 🚀 Installation

### Build from Source
```bash
git clone <repository-url>
cd reviwer
go build -o reviewer ./cmd/reviewer
```

### Binary Releases
Download pre-built binaries from the [releases page](releases).

## 📖 Usage

### Basic Command Structure
```bash
./reviewer [options]
```

### Review Modes

#### 1. Diff Review (Uncommitted Changes)
```bash
./reviewer -mode=diff-uncommitted -dir=./myproject
```

#### 2. Branch Comparison
```bash
./reviewer -mode=diff-branch -dir=./myproject -base=main
```

#### 3. Full Project Review
```bash
./reviewer -mode=review-project -dir=./myproject
```

#### 4. Single File Review
```bash
./reviewer -mode=review-file -file=./main.go
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|----------|
| `-config` | Path to configuration file | `config.toml` |
| `-mode` | Review mode (diff-uncommitted, diff-branch, review-project, review-file) | `diff-uncommitted` |
| `-dir` | Project directory for diff or review | `.` |
| `-file` | Single file to review | |
| `-base` | Base branch for diff-branch mode | `master` |
| `-write-tests` | Automatically write and run generated tests | `false` |
| `-keep-tests` | Keep generated test files after run | `false` |
| `-max-retries` | Max retries for failed chunks | `3` |
| `-chunk-timeout` | Timeout for each review chunk | `5m` |
| `-failed-chunks-file` | File to save/read failed chunk indices | `failed_chunks.json` |
| `-resume-failed` | Only process failed chunks from failed-chunks-file | `false` |
| `-llm-provider` | LLM provider: openai or lmstudio (overrides config) | |
| `-llm-model` | LLM model name (overrides config) | |

## ⚙️ Configuration

The tool uses a TOML configuration file (`config.toml`) with the following structure:

```toml
[llm]
provider = "openai"  # or "lmstudio"
model = "gpt-4o"

[review]
chunk_size = 1200
max_retries = 3
chunk_timeout = "5m"
write_tests = false
keep_tests = false

[languages.go]
extension = ".go"
review_prompt = "You are a programming expert reviewing Go code..."
test_prompt = "You are a testing expert. Generate unit tests..."

[languages.php]
extension = ".php"
review_prompt = "You are a programming expert reviewing PHP code..."
test_prompt = "You are a testing expert. Generate PHPUnit tests..."
```

### Environment Variables

- `OPENAI_API_KEY`: Required when using OpenAI provider

## 🏗️ Architecture

The project follows a clean, modular architecture:

```
reviewer/
├── cmd/reviewer/          # Main application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── diff/             # Git diff and chunking operations
│   ├── llm/              # LLM client implementations
│   └── review/           # Core review logic
├── pkg/types/            # Shared types and interfaces
└── config.toml           # Configuration file
```

### Key Components

- **Config Manager**: Handles TOML configuration loading and validation
- **Diff Provider**: Git operations and intelligent code chunking
- **LLM Client**: Multi-provider LLM integration with retry logic
- **Review Service**: Orchestrates the review process with progress tracking

## License
This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Contributing
Pull requests and issues are welcome!

## Author
[disconnekt](https://nalekseev.xyz)
