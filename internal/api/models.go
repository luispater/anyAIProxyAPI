package api

import (
	"github.com/gin-gonic/gin"
	"github.com/luispater/anyAIProxyAPI/internal/runner"
	"time"
)

type Prompt struct {
	Role        string
	ContentType string
	Content     string
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// RequestTask represents a queued request task
type RequestTask struct {
	ID           string             `json:"id"`
	Request      string             `json:"request"`
	Response     chan *TaskResponse `json:"-"`
	CreatedAt    time.Time          `json:"created_at"`
	Context      *gin.Context       `json:"context"`
	InstanceName string             `json:"instance_name"`
}

// TaskResponse represents the response from processing a task
type TaskResponse struct {
	Success  bool        `json:"success"`
	Response string      `json:"response,omitempty"`
	Stream   chan string `json:"-"`
	Error    error       `json:"error,omitempty"`
	Runner   *runner.RunnerManager
}
