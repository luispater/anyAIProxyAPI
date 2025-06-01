package api

import (
	"context"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/adapter"
	"github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	"github.com/luispater/anyAIProxyAPI/internal/runner"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"math/rand"
	"strings"
	"time"
)

// ChatProcessor implements TaskProcessor interface
type ChatProcessor struct {
	pages     map[string]*chrome.Page
	debug     bool
	appConfig *config.AppConfig
}

// NewChatProcessor creates a new chat processor
func NewChatProcessor(appConfig *config.AppConfig, pages map[string]*chrome.Page, debug bool) *ChatProcessor {
	return &ChatProcessor{
		pages:     pages,
		debug:     debug,
		appConfig: appConfig,
	}
}

// ProcessTask processes a chat completion task
func (cp *ChatProcessor) ProcessTask(ctx context.Context, task *RequestTask) *TaskResponse {
	log.Debugf("Starting to process task %s", task.ID)

	instanceName := ""
	modelResult := gjson.Get(task.Request, "model")
	if modelResult.Type == gjson.String {
		modelNames := strings.SplitN(modelResult.String(), "/", 2)
		if len(modelNames) == 2 {
			instanceName = modelNames[0]
			modelName := modelNames[1]
			task.Request, _ = sjson.Set(task.Request, "model", modelName)
		}
	}

	var appConfigRunner config.AppConfigRunner
	for i := 0; i < len(cp.appConfig.Instance); i++ {
		if cp.appConfig.Instance[i].Name == instanceName {
			appConfigRunner = cp.appConfig.Instance[i].Runner
		}
	}

	streamResult := gjson.Get(task.Request, "stream")
	if streamResult.Type == gjson.True {
		return cp.processStreamingTask(instanceName, appConfigRunner, ctx, task)
	} else {
		return cp.processNonStreamingTask(instanceName, appConfigRunner, ctx, task)
	}
}

// processNonStreamingTask processes a non-streaming request
func (cp *ChatProcessor) processNonStreamingTask(instanceName string, appConfigRunner config.AppConfigRunner, ctx context.Context, task *RequestTask) *TaskResponse {

	var fullResponse strings.Builder
	var done bool
	channel := make(chan *adapter.AdapterResponse)
	errChannel := make(chan error)
	streamChan := make(chan string, 100)

	page := cp.pages[instanceName]
	r, errNewRunnerManager := runner.NewRunnerManager(instanceName, appConfigRunner, page, cp.debug)
	go func() {
		if errNewRunnerManager != nil {
			log.Debug(errNewRunnerManager)
			return
		}
		r.SetVariable("REQUEST", task.Request, "string")
		r.SetVariable("PAGE", page, "ptr")
		r.SetVariable("PAGE-DATA-CHANNEL", channel, "ptr")
		err := r.Run("chat_completions")
		if err != nil {
			errChannel <- err
			log.Debug(err)
			return
		}
		log.Debug("all of the rules are executed.")
	}()

	go func() {
		defer close(streamChan)
		for !done {
			select {
			case err := <-errChannel:
				streamChan <- fmt.Sprintf(`{"error": %v}`, err)
				return
			case <-ctx.Done():
				return
			case data := <-channel:
				done = data.Done
				if data.Done {
					randomStr := generateRandomString(7)
					timestamp := time.Now().Unix()
					chatCmplId := fmt.Sprintf("chatcmpl-%s-%d", randomStr, timestamp)

					jsonTemplate := `{"id":"","object":"chat.completion","created":123456,"model":"model","choices":[{"index":0,"message":{"role":"assistant","content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null,"native_finish_reason":null}]}`
					jsonOutput, _ := sjson.Set(jsonTemplate, "id", chatCmplId)
					jsonOutput, _ = sjson.Set(jsonOutput, "created", timestamp)

					if data.Content != "" {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.message.content", data.Content)
					} else {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.message.content", nil)
					}

					if data.ReasoningContent != "" {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.message.reasoning_content", data.ReasoningContent)
					} else {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.message.reasoning_content", nil)
					}

					if data.ToolCalls != "" {
						jsonOutput, _ = sjson.SetRaw(jsonOutput, "choices.0.message.tool_calls", data.ToolCalls)
					} else {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.message.tool_calls", nil)
					}

					jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.finish_reason", "stop")
					jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.native_finish_reason", "stop")

					streamChan <- jsonOutput
					break
				}
			}
		}

		// Create response
		response := fullResponse.String()
		if r.NeedReportToken("chat_completions") {
			promptTokens, completionTokens, totalTokens := r.GetTokenReport()
			response, _ = sjson.Set(response, "usage.prompt_tokens", promptTokens)
			response, _ = sjson.Set(response, "usage.completion_tokens", completionTokens)
			response, _ = sjson.Set(response, "usage.total_tokens", totalTokens)
		}
	}()

	return &TaskResponse{
		Success: true,
		Stream:  streamChan,
		Runner:  r,
	}
}

// processStreamingTask processes a streaming request
func (cp *ChatProcessor) processStreamingTask(instanceName string, appConfigRunner config.AppConfigRunner, ctx context.Context, task *RequestTask) *TaskResponse {
	// Create streaming channel
	streamChan := make(chan string, 100)
	channel := make(chan *adapter.AdapterResponse)
	errChannel := make(chan error)

	page := cp.pages[instanceName]
	r, errNewRunnerManager := runner.NewRunnerManager(instanceName, appConfigRunner, page, cp.debug)
	go func() {
		if errNewRunnerManager != nil {
			log.Debug(errNewRunnerManager)
			return
		}
		r.SetVariable("REQUEST", task.Request, "string")
		r.SetVariable("PAGE", page, "ptr")
		r.SetVariable("PAGE-DATA-CHANNEL", channel, "ptr")
		err := r.Run("chat_completions")
		if err != nil {
			errChannel <- err
			log.Debug(err)
			return
		}
		log.Debugf("all of the rules are executed.")
	}()

	// Start streaming goroutine
	go func() {
		defer close(streamChan)
		isFirst := true
		lastContext := ""
		lastReasoningContent := ""

		randomStr := generateRandomString(7)
		timestamp := time.Now().Unix()
		chatCmplId := fmt.Sprintf("chatcmpl-%s-%d", randomStr, timestamp)

		jsonTemplate := `{"id":"","object":"chat.completion.chunk","created":12345,"model":"model","choices":[{"index":0,"delta":{"role":null,"content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null,"native_finish_reason":null}]}`
		jsonTemplate, _ = sjson.Set(jsonTemplate, "id", chatCmplId)
		jsonTemplate, _ = sjson.Set(jsonTemplate, "created", timestamp)

		var done bool
		for !done {
			select {
			case err := <-errChannel:
				streamChan <- fmt.Sprintf(`{"error": %v}`, err)
				return
			case <-ctx.Done():
				return
			case data := <-channel:
				done = data.Done
				jsonOutput := ""

				if len(data.ReasoningContent) > len(lastReasoningContent) {
					jsonOutput, _ = sjson.Set(jsonTemplate, "choices.0.delta.reasoning_content", data.ReasoningContent[len(lastReasoningContent):])
					lastReasoningContent = data.ReasoningContent
				} else if len(data.Content) > len(lastContext) {
					jsonOutput, _ = sjson.Set(jsonTemplate, "choices.0.delta.content", data.Content[len(lastContext):])
					lastContext = data.Content
				} else if data.Done {
					jsonOutput, _ = sjson.Set(jsonTemplate, "choices.0.finish_reason", "stop")
					jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.native_finish_reason", "stop")
					if len(data.ToolCalls) > 0 {
						jsonOutput, _ = sjson.SetRaw(jsonOutput, "choices.0.delta.tool_calls", data.ToolCalls)
					}

					if r.NeedReportToken("chat_completions") {
						promptTokens, completionTokens, totalTokens := r.GetTokenReport()
						jsonOutput, _ = sjson.Set(jsonOutput, "usage.prompt_tokens", promptTokens)
						jsonOutput, _ = sjson.Set(jsonOutput, "usage.completion_tokens", completionTokens)
						jsonOutput, _ = sjson.Set(jsonOutput, "usage.total_tokens", totalTokens)
					}
				}

				if jsonOutput != "" {
					if isFirst {
						jsonOutput, _ = sjson.Set(jsonOutput, "choices.0.delta.role", "assistant")
						isFirst = false
					}
					streamChan <- jsonOutput
				}
			}
		}
	}()

	return &TaskResponse{
		Success: true,
		Stream:  streamChan,
		Runner:  r,
	}
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
