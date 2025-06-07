# Reviewer: LLM-Powered Code Review and Test Suggestion Tool

Reviewer is a command-line tool that leverages Large Language Models (LLMs) to perform automated code review and generate unit test suggestions for your codebase. It supports both OpenAI and LM Studio backends, processes code in chunks, and provides detailed feedback and test files for each chunk.

## Features
- Automated code review using LLMs (OpenAI or LM Studio)
- Batch processing of large codebases (chunked review)
- Unit test generation and suggestion per code chunk
- Unique test file naming to avoid overwrites
- Robust error handling and retry logic
- Detailed logging and summary output
- Automatic cleanup of generated test files (optional)
- CLI integration test for reliability

## Usage

```
./reviewer -dir=<source_dir> -mode=review-project -llm-provider=<openai|lmstudio> -llm-model=<model_name> [options]
```

### Example
```
./reviewer -dir=../../myproject/ -mode=review-project -llm-provider=lmstudio -llm-model="claude-3.7-sonnet-reasoning-gemma3-12b"
```

### Options
- `--keep-tests`         Keep generated test files after review
- `--chunk-timeout`      Timeout per chunk (default: 60s)
- `--max-retries`        Max retries per chunk (default: 3)
- `--failed-chunks-file` Save failed chunks to a file for resuming

## Configuration
- Edit `config.toml` to set language prompts and model defaults.
- Set environment variables (e.g., `OPENAI_API_KEY`) as needed.

## License
This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Contributing
Pull requests and issues are welcome!

## Author
[disconnekt](https://nalekseev.xyz)
