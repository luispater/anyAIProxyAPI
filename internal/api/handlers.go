package api

import (
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/config"
	"github.com/luispater/anyAIProxyAPI/internal/runner"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var RequestMutex sync.Mutex
var ScreenshotMutex sync.Mutex

// APIHandlers contains the handlers for API endpoints
type APIHandlers struct {
	queue     *RequestQueue
	pages     map[string]playwright.Page
	debug     bool
	appConfig *config.AppConfig
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(appConfig *config.AppConfig, queue *RequestQueue, pages map[string]playwright.Page, debug bool) *APIHandlers {
	return &APIHandlers{
		queue:     queue,
		pages:     pages,
		debug:     debug,
		appConfig: appConfig,
	}
}

func (h *APIHandlers) TakeScreenshot(c *gin.Context) {
	defer ScreenshotMutex.Unlock()
	ScreenshotMutex.Lock()
	instanceName, ok := c.GetQuery("name")
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	if page, hasKey := h.pages[instanceName]; hasKey {
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		screenshot, err := page.Screenshot()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err), "code": 500})
		}
		_, _ = c.Writer.Write(screenshot)
	} else {
		c.Status(http.StatusNotFound)
	}
}

// ChatCompletions handles the /v1/chat/completions endpoint
func (h *APIHandlers) ChatCompletions(c *gin.Context) {
	defer RequestMutex.Unlock()
	RequestMutex.Lock()
	rawJson, err := c.GetRawData()
	// If data retrieval fails, return 400 error
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err), "code": 400})
		return
	}

	instanceName := ""
	modelResult := gjson.GetBytes(rawJson, "model")
	if modelResult.Type == gjson.String {
		instanceName = strings.Split(modelResult.String(), "/")[0]
	}

	instanceIndex := 0
	for i := 0; i < len(h.appConfig.Instance); i++ {
		if h.appConfig.Instance[i].Name == instanceName {
			instanceIndex = i
		}
	}

	// Generate unique task ID
	taskID := uuid.New().String()

	// Create a task
	task := &RequestTask{
		ID:           taskID,
		Request:      string(rawJson),
		Response:     make(chan *TaskResponse, 1),
		CreatedAt:    time.Now(),
		Context:      c,
		InstanceName: instanceName,
	}

	// Add a task to queue
	if err = h.queue.AddTask(task); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: ErrorDetail{
				Message: fmt.Sprintf("Failed to queue request: %v", err),
				Type:    "server_error",
			},
		})
		return
	}

	// Wait for response
	select {
	case response := <-task.Response:
		if !response.Success {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error: ErrorDetail{
					Message: fmt.Sprintf("Processing failed: %v", response.Error),
					Type:    "server_error",
				},
			})
			return
		}

		streamResult := gjson.GetBytes(rawJson, "stream")
		if streamResult.Type == gjson.True {
			h.handleStreamingResponse(instanceIndex, c, response)
		} else {
			h.handleNonStreamingResponse(instanceIndex, c, response)
		}

	case <-time.After(5 * time.Minute): // 5 minute timeout
		c.JSON(http.StatusRequestTimeout, ErrorResponse{
			Error: ErrorDetail{
				Message: "Request timeout",
				Type:    "timeout_error",
			},
		})
		return
	}
}

func (h *APIHandlers) handleContextCanceled(instanceIndex int) {
	page := h.pages[h.appConfig.Instance[instanceIndex].Name]
	r, err := runner.NewRunnerManager(h.appConfig.Instance[instanceIndex].Name, h.appConfig.Instance[instanceIndex].Runner, &page, h.debug)
	if err != nil {
		log.Error(err)
		return
	}
	err = r.Run("context_canceled")
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugf("all of the rules are executed.")
}

// handleNonStreamingResponse handles non-streaming responses
func (h *APIHandlers) handleNonStreamingResponse(instanceIndex int, c *gin.Context, response *TaskResponse) {

	c.Header("Content-Type", "application/json")

	// Handle streaming manually
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			if c.Request.Context().Err().Error() == "context canceled" {
				log.Debugf("Client disconnected: %v", c.Request.Context().Err())
				h.handleContextCanceled(instanceIndex)
				response.Runner.Abort()
			}
			return
		case chunk, okStream := <-response.Stream:
			if strings.HasPrefix(chunk, "{\"error\"") {
				c.Status(500)
				_, _ = fmt.Fprintf(c.Writer, chunk)
				flusher.Flush()
				return
			}

			if !okStream {
				return
			}

			c.Status(http.StatusOK)
			_, _ = fmt.Fprintf(c.Writer, "%s", chunk)
			flusher.Flush()
		case <-time.After(500 * time.Millisecond):
			// Write processing tag
			_, _ = c.Writer.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}

// handleStreamingResponse handles streaming responses
func (h *APIHandlers) handleStreamingResponse(instanceIndex int, c *gin.Context, response *TaskResponse) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Handle streaming manually
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			if c.Request.Context().Err().Error() == "context canceled" {
				log.Debugf("Client disconnected: %v", c.Request.Context().Err())
				h.handleContextCanceled(instanceIndex)
				response.Runner.Abort()
			}
			return
		case chunk, okStream := <-response.Stream:
			if strings.HasPrefix(chunk, "{\"error\"") {
				// Stream closed, send final message
				c.Status(500)
				_, _ = fmt.Fprintf(c.Writer, chunk)
				flusher.Flush()
				return
			}

			if !okStream {
				// Stream closed, send final message
				_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			// Send chunk as SSE
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", chunk)
			flusher.Flush()

		case <-time.After(300 * time.Second):
			// Timeout, close stream
			_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		case <-time.After(500 * time.Millisecond):
			// Write processing tag
			_, _ = c.Writer.Write([]byte(": ANY-AI-PROXY-API PROCESSING\n\n"))
			flusher.Flush()
		}
	}
}
