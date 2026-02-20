# Hawkeye CLI

Command-line client for the [Neubird Hawkeye](https://neubird.com) AI SRE platform. Run AI-powered incident investigations, stream chain-of-thought reasoning, and review session results — all from your terminal.

## Quick Start

### Install

Golang:

```
go install github.com/neubirdai/hawkeye-cli@latest'
```

macOS:

```
brew tap neubirdai/hawkeye
brew install neubird-hawkeye
```

Running from Go without install:

```bash
alias hawkeye='go run github.com/neubirdai/hawkeye-cli@latest'
```

### Usage

#### Login 
hawkeye login https://your-hawkeye.app.neubird.ai -u you@company.com -p 'your-password'

#### List Projects
hawkeye projects

#### Set project
hawkeye set project <your-project-uuid>

#### Investigate
hawkeye ask "Why is the API returning 500 errors?"
```

