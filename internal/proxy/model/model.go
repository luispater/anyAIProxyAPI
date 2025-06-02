package model

type ProxyResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        string
	Done             bool
}
