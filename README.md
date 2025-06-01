# Any Proxy API (Go)

A Go-based proxy server that provides OpenAI-compatible API endpoints for **Any** AI website using browser automation.

## Overview

This project creates a bridge between OpenAI's API format and an AI website's web interface. It uses Playwright for browser automation to interact with **Any** AI website and provides a REST API that mimics OpenAI's chat completions endpoint.

## Features

- **OpenAI-Compatible API**: Supports `/v1/chat/completions` endpoints
- **Browser Automation**: Uses Playwright with [Camoufox](https://github.com/daijro/camoufox) browser for web automation
- **Request Queue**: Implements a queue system to handle requests sequentially
- **Configurable Workflows**: YAML-based configuration for different automation workflows
- **Proxy Support**: Built-in HTTP proxy for network traffic interception
- **Multi-Instance Support**: Can manage multiple AI Studio instances simultaneously

## Architecture

The application consists of several key components:

### Core Components

1. **API Server** (`internal/api/`): Gin-based HTTP server providing OpenAI-compatible endpoints
2. **Browser Manager** (`internal/browser/`): Manages Playwright browser instances and contexts
3. **Runner System** (`internal/runner/`): Executes YAML-defined workflows for browser automation
4. **Method Library** (`internal/method/`): Collection of automation methods (click, input, etc.)
5. **Proxy Server** (`internal/proxy/`): HTTP proxy for intercepting and analyzing network traffic
6. **Configuration** (`internal/config/`): Application configuration management

### Request Flow

1. Client sends OpenAI-format request to `/v1/chat/completions`
2. Request is queued in the request queue system
3. Runner executes the appropriate YAML workflow to interact with AI Studio
4. Browser automation performs the necessary actions (input text, click buttons, etc.)
5. Proxy intercepts the response from AI Studio
6. Response is formatted and returned to the client

## Installation

### Prerequisites

- Go 1.24 or later
- Camoufox browser

### Setup

1. Clone the repository:
```bash
git clone https://github.com/luispater/anyAIProxyAPI.git
cd anyAIProxyAPI
```

2. Install dependencies:
```bash
go mod download
```

3. Install Playwright browsers:
```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install
```

4. Configure the application by editing `runner/main.yaml`

## Configuration

The main configuration file is `runner/main.yaml`:

```yaml
version: "1"
debug: false
camoufox-path: "/path/to/camoufox"
api-port: "2048"
headless: true
instance:
  - name: "example"
    proxy-url: ""
    url: "https://example.com/new_chat"
    sniff-port: "3120"
    sniff-domain: "*.example.com"
    auth-file: "auth/example.json"
    runner: # must be init, chat_completions, context_canceled
      init: "init-system" #init runner
      chat_completions: "chat_completions" #chat_completions runner
      context_canceled: "context-canceled" #context canceled(client disconnect) runner
```

### Configuration Parameters

- `debug`: Enable debug mode for detailed logging
- `camoufox-path`: Path to Camoufox browser executable
- `api-port`: Port for the API server
- `headless`: Run browser in headless mode
- `instance`: Array of AI Studio instances to manage. Each instance has its own configuration. All runner files must be defined in a directory corresponding to the instance name. For details on the runner file syntax, please refer to [runner.md](runner.md)

## Usage

### Starting the Server

```bash
go run main.go
```

The server will start on the configured port (default: 2048).

### API Endpoints

#### Chat Completions
```bash
POST http://localhost:2048/v1/chat/completions
Content-Type: application/json

{
  "model": "instance-name/model-name",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ]
}
```

## Workflow System

The application uses a YAML-based workflow system to define browser automation sequences. Workflows are stored in the `runner/` directory and define step-by-step instructions for interacting with AI Studio.

For detailed information about the runner system, see [runner.md](runner.md).

## Development

### Project Structure

```
├── main.go                 # Application entry point
├── internal/
│   ├── api/               # HTTP API server
│   ├── browser/           # Browser management
│   ├── config/            # Configuration handling
│   ├── method/            # Automation methods
│   ├── proxy/             # HTTP proxy server
│   └── runner/            # Workflow execution engine
├── runner/
│   ├── main.yaml          # Main configuration
│   └── instance-name/     # AI Studio workflows
├── auth/                  # Authentication files
└── docs/                  # Documentation
```

### Building

```bash
go build -o any-ai-proxy main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License. Refer to the LICENSE file for details.

## Acknowledgements

This project was inspired by [AIStudioProxyAPI](https://github.com/CJackHwang/AIstudioProxyAPI)

## Disclaimer

This project is for educational and research purposes. Please ensure you comply with **Any** AI website's terms of service when using this software.
