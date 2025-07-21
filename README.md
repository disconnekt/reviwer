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

#### 5. Using Different LLM Providers

**Claude AI:**
```bash
export CLAUDE_API_KEY="your-claude-api-key"
./reviewer -mode=review-project -llm-provider=claude -llm-model="claude-3-sonnet-20240229"
```

**Mistral AI:**
```bash
export MISTRAL_API_KEY="your-mistral-api-key"
./reviewer -mode=review-project -llm-provider=mistral -llm-model="mistral-large-latest"
```

**Groq:**
```bash
export GROQ_API_KEY="your-groq-api-key"
./reviewer -mode=review-project -llm-provider=groq -llm-model="llama3-70b-8192"
```

**LM Studio (Local):**
```bash
./reviewer -mode=review-project -llm-provider=lmstudio -llm-model="your-local-model"
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
| `-llm-provider` | LLM provider: openai, lmstudio, claude, mistral, groq | `openai` |
| `-llm-model` | LLM model name (overrides config) | |

### Environment Variables

- `OPENAI_API_KEY`: Required when using OpenAI provider
- `CLAUDE_API_KEY`: Required when using Claude provider
- `MISTRAL_API_KEY`: Required when using Mistral provider
- `GROQ_API_KEY`: Required when using Groq provider

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
[Nikita Alekseev](https://nalekseev.xyz)
