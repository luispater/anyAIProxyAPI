package model

type ProxyResponse struct {
	Context          string
	ReasoningContent string
	ToolCalls        string
	Done             bool
}
