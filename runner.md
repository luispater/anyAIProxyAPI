# Runner System Documentation

The Runner system is the core automation engine of Any AI Proxy API. It executes YAML-defined workflows to interact with **Any** AI website through browser automation.

## Overview

The Runner system consists of two main parts:

1. **Runner Engine** (`internal/runner/`): The execution engine that processes YAML workflows
2. **Method Library** (`internal/method/`): A collection of automation methods that can be called from workflows

## Architecture

### Runner Manager (`internal/runner/manager.go`)

The `RunnerManager` is the central component that:

- Loads and manages YAML configuration files
- Executes workflows step by step
- Manages variables and results between workflow steps
- Handles error conditions and retry logic
- Provides debugging capabilities

Key features:
- **Dynamic Configuration Loading**: Automatically scans and loads YAML files from `runner/{instance-name}/` directory
- **Variable Management**: Stores and retrieves variables between workflow steps using `#VARIABLE_NAME#` syntax
- **Error Handling**: Supports retry mechanisms and fallback actions
- **Nested Workflows**: Supports sub-workflows and conditional execution

### Method Library (`internal/method/`)

The method library provides a comprehensive set of automation methods organized by functionality:

#### Browser Interaction Methods

**Element Operations** (`element.go`):
- `GetElement(selector)`: Find a single element by CSS selector
- `GetElements(selector)`: Find multiple elements by CSS selector
- `GetElementAttribute(locator, attribute)`: Get element attribute value
- `GetInnerText(selector)`: Get the inner text content of an element

**Mouse Operations** (`mouse.go`):
- `Click(selector, timeout)`: Click on an element
- `MouseClick(x, y)`: Click at specific coordinates

**Input Operations** (`input.go`):
- `Input(selector, text, timeout)`: Input text into an element
- `Value(selector, timeout)`: Get input element value

**Visibility Checks** (`check.go`):
- `IsVisible(element)`: Check if an element is visible
- `IsDisabled(element)`: Check if an element is disabled

#### Request Processing Methods

**Request Analysis** (`request.go`):
- `Temperature(requestJson)`: Extract temperature parameter from request
- `TopP(requestJson)`: Extract top_p parameter from request
- `StopSequence(requestJson)`: Extract stop sequences from request
- `MaxTokens(requestJson)`: Extract max_tokens parameter from request
- `PromptCount(requestJson)`: Count messages by role (system, user, assistant, tool)
- `SystemPrompt(requestJson)`: Extract system prompt from request
- `UserPrompt(requestJson)`: Extract user prompt from request
- `ImagePrompt(requestJson)`: Extract image URLs from request messages
- `ToolPrompt(requestJson)`: Extract tool/function call information from request
- `Model(requestJson)`: Extract model name from request

#### File Operations

**File Upload** (`file.go`):
- `UploadFiles(runner, base64Images)`: Upload files through file chooser dialog
- Supports various MIME types and file formats
- Handles base64-encoded file data

#### Network Operations

**Proxy Integration** (`sniff.go`):
- `StartSniffing(proxy)`: Start network traffic monitoring
- `StopSniffing(proxy)`: Stop network traffic monitoring
- `GetDataFromProxy(proxy, channel)`: Retrieve intercepted data

#### Utility Methods

**String Operations** (`string.go`):
- `StringEqual(str1, str2)`: Compare two strings

**Integer Operations** (`int.go`):
- `IsEqual(a, b)`: Compare two integers

**Mathematical Operations** (`math.go`):
- `Gt(a, b)`: Greater than comparison
- `Gte(a, b)`: Greater than or equal comparison
- `Lt(a, b)`: Less than comparison
- `Lte(a, b)`: Less than or equal comparison
- `Eq(a, b)`: Equality comparison

**Control Flow** (`syntax.go`):
- `SleepMilliseconds(ms)`: Pause execution

**Tools** (`tools.go`):
- `AlwaysTrue()`: Always returns true (for some special workflow)
- `GetLocalStorage(name)`: Get browser local storage value
- `SetLocalStorage(name, value)`: Set browser local storage value
- `Int(i)`: Convert to integer
- `Len(arr)`: Get array length

## YAML Workflow Configuration

### Configuration Structure

Each YAML workflow file follows this structure:

```yaml
version: "1"
name: "workflow_name"
need_report_token: false  # Optional: whether to report token usage
workflow:
  - index: 100
    action: "MethodName"
    description: "Description of this step"
    params:
      - "parameter1"
      - "parameter2"
    retry: 3  # Optional: number of retries
    result:
      - result_index: 0
        name: "variableName"  # Optional: store result in variable
        type: "string"        # Result type: string, int, bool, error, etc.
        policy:
          has_error: "FAILBACK" # Action on error, the value FAILBACK does failback action
          no_error: "CONTINUE" # Action on success, the value CONTINUE does the next step
          is_true: "BREAK" # Action when a result is true, the value BREAK does break the current workflow
          is_false: "FAILED" # Action when a result is false, the value FAILED does stop the runner(or sub-runner)
    failback:  # Optional: fallback action on failure
      action: "Click"
      params:
        - "selector"
        - "timeout"
    workflow:  # Optional: sub-workflow
      - index: 0
        action: "SubAction"
        # ... sub-workflow steps
```

### Variable System

The Runner system supports a powerful variable system:

#### Variable Declaration
Variables are automatically created when methods return values and are assigned names in the `result` section.

#### Variable Usage
Variables can be referenced in parameters using the `#VARIABLE_NAME#` syntax:

```yaml
- index: 100
  action: "GetElement"
  params:
    - "button.submit"
  result:
    - result_index: 0
      name: "submitButton" # define a variable use the name tag, if you don't define the name, it will not store the result
      type: "object"

- index: 200
  action: "Click"
  params:
    - "#submitButton#"  # Use the previously stored element
    - "2500"
```

#### Special Variables
- `#REQUEST#`: Contains the original API request JSON
- `#NEW_RUNNER#`: Creates a new runner instance
- `#PROXY#`: Reference to the proxy instance
- `#PROXY-DATA-CHANNEL#`: Channel for proxy data communication

### Control Flow

#### Conditional Execution
Use policies to control workflow execution based on results:

```yaml
result:
  - result_index: 0
    type: "bool"
    policy:
      is_true: "CONTINUE"      # Continue to the next step
      is_false: "FAILED"       # Stop the runner(or sub-runner)
  - result_index: 1
    type: "error"
    policy:
      no_error: "BREAK"        # Stop the current workflow
      has_error: "FAILBACK"    # Failback action
```

#### Sub-workflows
Execute nested workflows conditionally:

```yaml
result:
  - result_index: 0
    type: "error"
    policy:
      no_error: "DO-WORKFLOW"  # Execute sub-workflow on success
workflow:
  - index: 0
    action: "SubAction"
    # ... sub-workflow steps
```

#### Error Handling
Handle errors with fallback actions:

```yaml
failback:
  action: "Click"
  description: "Fallback: click home button"
  params:
    - 'a[href="/"]'
    - "2500"
```

## Workflow Examples

### Example 1: Basic Element Interaction

```yaml
version: "1"
name: "click_button"
workflow:
  - index: 100
    action: "Click"
    description: "Click the submit button"
    params:
      - "button.submit"
      - "2500"
    result:
      - result_index: 0
        type: "error"
        policy:
          has_error: "FAILED"
          no_error: "CONTINUE"
```

### Example 2: Request Processing

```yaml
version: "1"
name: "process_request"
workflow:
  - index: 100
    action: "UserPrompt"
    description: "Extract user prompt from request"
    params:
      - "#REQUEST#"
    result:
      - result_index: 0
        type: "bool"
        policy:
          is_true: "DO-WORKFLOW"
          is_false: "BREAK"
      - result_index: 1
        name: "userMessage"
        type: "string"
    workflow:
      - index: 0
        action: "Input"
        description: "Input the user message"
        params:
          - "textarea.prompt-input"
          - "#userMessage#"
          - "5000"
```

### Example 3: Model Selection

```yaml
version: "1"
name: "choose_model"
workflow:
  - index: 100
    action: "Model"
    description: "Get the request model name"
    params:
      - "#REQUEST#"
    result:
      - result_index: 0
        type: "bool"
        policy:
          is_true: "CONTINUE"
          is_false: "BREAK"
      - result_index: 1
        name: "modelName"
        type: "string"
  - index: 200
    action: "Click"
    description: "Click the model selector"
    params:
      - "div.model-selector"
      - "2500"
  - index: 300
    action: "ChooseModelByName"
    description: "Select model by name"
    params:
      - "#modelName#"
      - "div.model-option"
      - "div.model-container"
```

## Execution Flow

### 1. Initialization
When a runner is created:
1. Load all YAML files from the instance directory
2. Parse and validate workflow configurations
3. Initialize the method library with the browser page instance

### 2. Workflow Execution
When `Run(workflowName)` is called:
1. Find the specified workflow configuration
2. Execute workflow steps in order by index
3. Handle results and apply policies
4. Manage variables and state
5. Execute sub-workflows or fallbacks as needed

### 3. Method Invocation
For each workflow step:
1. Resolve parameter variables
2. Find the corresponding method in the method library
3. Convert parameters to appropriate types
4. Invoke the method using reflection
5. Process return values and store results

### 4. Error Handling
When errors occur:
1. Check retry count and retry if possible
2. Execute fallback actions if defined
3. Apply error policies (FAILED, CONTINUE, etc.)
4. Propagate errors up the workflow chain

## Integration with API

The Runner system integrates with the API layer through:

1. **Request Processing**: API handlers create runner instances and pass request data
2. **Variable Injection**: Request JSON and proxy instances are injected as variables
3. **Asynchronous Execution**: Runners execute in goroutines to handle concurrent requests
4. **Result Collection**: API handlers collect results through channels and proxy data

## Debugging and Development

### Debug Mode
Enable debug mode in configuration to:
- Reload YAML files on each execution
- Enable detailed logging
- Store configuration file paths for reference

### Logging
The system provides comprehensive logging at different levels:
- Debug: Detailed execution information
- Info: General workflow progress
- Warn: Non-fatal issues
- Error: Critical failures

### Testing Workflows
Individual workflows can be tested by:
1. Creating minimal YAML configurations
2. Using the `AlwaysTrue()` method for testing control flow
3. Enabling debug mode for detailed output
4. Using browser developer tools to inspect element selectors

This Runner system provides a flexible and powerful foundation for automating complex browser interactions with Google AI Studio while maintaining clean separation between automation logic and application code.
