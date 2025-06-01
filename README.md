# Any AI Proxy API (Go)

A Go-based proxy server that provides OpenAI-compatible API endpoints for **Any** AI website using browser automation.

## Overview

This project creates a bridge between OpenAI's API format and various AI websites' web interfaces. It uses ChromeDP for browser automation to interact with **Any** AI website and provides a REST API that mimics OpenAI's chat completions endpoint.

## Features

- **OpenAI-Compatible API**: Supports `/v1/chat/completions` endpoints
- **Browser Automation**: Uses ChromeDP with [Fingerprint Chromium](https://github.com/adryfish/fingerprint-chromium) browser for web automation
- **Request Queue**: Implements a queue system to handle requests sequentially
- **Configurable Workflows**: YAML-based configuration for different automation workflows
- **Multi-AI Service Support**: Supports ChatGPT, Gemini AI Studio, Grok, and more
- **Multi-Instance Support**: Can manage multiple AI service instances simultaneously
- **Screenshot API**: Built-in screenshot functionality for debugging
- **Authentication Management**: Automatic cookie and session management

## Supported AI Services

Currently supports the following AI services:

- **ChatGPT** (https://chatgpt.com/)
- **Gemini AI Studio** (https://aistudio.google.com/)
- **Grok** (https://grok.com/)

Each service has a dedicated adapter to handle its specific response format and interaction patterns.

## Architecture

The application consists of several key components:

### Core Components

1. **API Server** (`internal/api/`): Gin-based HTTP server providing OpenAI-compatible endpoints
2. **Browser Manager** (`internal/browser/chrome/`): Manages ChromeDP browser instances and contexts
3. **Runner System** (`internal/runner/`): Executes YAML-defined workflows for browser automation
4. **Method Library** (`internal/method/`): Collection of automation methods (click, input, etc.)
5. **Adapter System** (`internal/adapter/`): Handles response format conversion for different AI services
6. **Configuration** (`internal/config/`): Application configuration management
7. **Utils** (`internal/utils/`): Common utility functions

### Request Flow

1. Client sends OpenAI-format request to `/v1/chat/completions`
2. Request is queued in the request queue system
3. Runner executes the appropriate YAML workflow to interact with the AI service
4. Browser automation performs the necessary actions (input text, click buttons, etc.)
5. Adapter intercepts and processes the response from the AI service
6. Response is formatted and returned to the client

## Installation

### Prerequisites

- Go 1.24 or later
- Fingerprint Chromium browser

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

3. Configure the application by editing `runner/main.yaml`

## Configuration

The main configuration file is `runner/main.yaml`:

```yaml
version: "1"
debug: true
browser:
  fingerprint-chromium-path: "/Applications/Chromium.app/Contents/MacOS/Chromium"
  args:
    - "--fingerprint=1000"
    - "--timezone=America/Los_Angeles"
    - "--remote-debugging-port=9222"
    - "--lang=en-US"
    - "--accept-lang=en-US"
  user-data-dir: "/anyAIProxyAPI/user-data-dir"
api-port: "2048"
headless: false
instance:
  - name: "gemini-aistudio"
    adapter: "gemini-aistudio"
    proxy-url: ""
    url: "https://aistudio.google.com/prompts/new_chat"
    sniff-url:
      - "https://alkalimakersuite-pa.clients6.google.com/$rpc/google.internal.alkali.applications.makersuite.v1.MakerSuiteService/GenerateContent"
    auth:
      file: "auth/gemini-aistudio.json"
      check: "ms-settings-menu"
    runner: # must be init, chat_completions, context_canceled
      init: "init-system" # init runner
      chat_completions: "chat_completions" # chat_completions runner
      context_canceled: "context-canceled" # context canceled(client disconnect) runner
  - name: "chatgpt"
    adapter: "chatgpt"
    proxy-url: ""
    url: "https://chatgpt.com/"
    sniff-url:
      - "https://chatgpt.com/backend-api/conversation"
    auth:
      file: "auth/chatgpt.json"
      check: 'div[id="sidebar-header"]'
    runner:
      init: "init"
      chat_completions: "chat_completions"
      context_canceled: "context-canceled"
  - name: "grok"
    adapter: "grok"
    proxy-url: ""
    url: "https://grok.com/"
    sniff-url:
      - "https://grok.com/rest/app-chat/conversations/new"
    auth:
      file: "auth/grok.json"
      check: 'a[href="/chat#private"]'
    runner:
      init: "init-system"
      chat_completions: "chat_completions"
      context_canceled: "context-canceled"
```

### Configuration Parameters

- `debug`: Enable debug mode for detailed logging
- `browser`: Browser executable settings
  - `fingerprint-chromium-path`: Path to Fingerprint Chromium browser
  - `args`: Browser launch arguments
  - `user-data-dir`: User data directory
- `api-port`: Port for the API server
- `headless`: Run browser in headless mode
- `instance`: Array of AI service instances to manage. Each instance has its own configuration
  - `name`: Instance name
  - `adapter`: Adapter name (corresponds to different AI services)
  - `url`: AI service URL
  - `sniff-url`: URL patterns for intercepting responses
  - `auth`: Authentication configuration
    - `file`: File to store authentication information
    - `check`: CSS selector to check login status
  - `runner`: Runner configuration. All runner files must be defined in a directory corresponding to the instance name

For details on the runner file syntax, please refer to [runner.md](runner.md)

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

#### Headless Screenshot
```bash
GET http://localhost:2048/screenshot?instance=instance-name
```

#### Server Information
```bash
GET http://localhost:2048/
```

### Usage Examples

#### Interacting with ChatGPT
```bash
curl -X POST http://localhost:2048/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "chatgpt/gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Explain the basic principles of quantum computing"
      }
    ]
  }'
```

#### Interacting with Gemini
```bash
curl -X POST http://localhost:2048/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini/gemini-pro",
    "messages": [
      {
        "role": "user",
        "content": "Write a Python quicksort algorithm"
      }
    ]
  }'
```

#### Interacting with Grok
```bash
curl -X POST http://localhost:2048/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok/grok3",
    "messages": [
      {
        "role": "user",
        "content": "What are the latest developments in AI?"
      }
    ]
  }'
```

## Workflow System

The application uses a YAML-based workflow system to define browser automation sequences. Workflows are stored in the `runner/` directory and define step-by-step instructions for interacting with AI services.

### Workflow Structure

Each AI service instance has its own workflow directory:
- `runner/instance-name/` - Any AI website related workflows

Each directory contains the following core workflow files:
- `init.yaml` or `init-system.yaml` - Initialization workflow
- `chat_completions.yaml` - Chat completion workflow
- `context-canceled.yaml` - Context cancellation workflow

For detailed information about the runner system, see [runner.md](runner.md).

## Development

### Project Structure

```
├── main.go                    # Application entry point
├── go.mod                     # Go module file
├── go.sum                     # Go dependency checksum file
├── LICENSE                    # MIT license
├── README.md                  # Project documentation
├── runner.md                  # Runner system documentation
├── internal/                  # Internal packages
│   ├── adapter/               # AI website adapters
│   │   ├── adapter.go         # Adapter interface
│   │   ├── chatgpt.go         # ChatGPT adapter
│   │   ├── gemini-aistudio.go # Gemini AI Studio adapter
│   │   └── grok.go            # Grok adapter
│   ├── api/                   # HTTP API server
│   │   ├── server.go          # Server main
│   │   ├── handlers.go        # API handlers
│   │   ├── queue.go           # Request queue
│   │   └── processor.go       # Chat processor
│   ├── browser/               # Browser management
│   │   └── chrome/            # ChromeDP manager
│   ├── config/                # Configuration handling
│   ├── method/                # Automation methods
│   ├── runner/                # Workflow execution engine
│   └── utils/                 # Utility functions
├── runner/                    # Workflow configurations
│   ├── main.yaml              # Main configuration file
│   └── instance-name/         # Instance workflows
└── auth/                      # Authentication files
```

### Building

```bash
go build -o any-ai-proxy main.go
```

### Running Tests

```bash
go test ./...
```

## Technology Stack

- **Go 1.24+**: Main programming language
- **ChromeDP**: Browser automation library
- **Gin**: HTTP web framework
- **YAML**: Configuration file format
- **Logrus**: Structured logging library

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License. Refer to the [LICENSE](LICENSE) file for details.

## Acknowledgements

This project was inspired by [AIStudioProxyAPI](https://github.com/CJackHwang/AIstudioProxyAPI)

## Disclaimer

This project is for educational and research purposes. Please ensure you comply with **Any** AI website's terms of service when using this software.

## FAQ

### Q: How to add support for a new AI service?
A: You need to create a new adapter (in `internal/adapter/`) and corresponding workflow configurations (in `runner/` directory).

### Q: What to do if the browser fails to start?
A: Please check if the Fingerprint Chromium path configuration is correct and ensure the browser executable exists.

### Q: How to debug workflows?
A: Set `debug: true` in `runner/main.yaml`, which will enable detailed debug logging.

### Q: Which operating systems are supported?
A: Supports macOS, Linux, and Windows, but requires the corresponding platform's Fingerprint Chromium browser.
