package adapter

type AdapterResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        string
	Done             bool
}

var Adapters = map[string]Adapter{}

type Adapter interface {
	HandleResponse(responseBuffer []byte, done bool) (*AdapterResponse, error)
}
