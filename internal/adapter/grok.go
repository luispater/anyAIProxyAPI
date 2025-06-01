package adapter

import (
	"github.com/tidwall/gjson"
	"strings"
)

func init() {
	Adapters["grok"] = &GrokAdapter{}
}

type GrokAdapter struct {
}

func (g *GrokAdapter) HandleResponse(responseBuffer []byte, done bool) (*AdapterResponse, error) {
	think := ""
	body := ""
	toolCalls := ""

	parsedObjects := strings.Split(string(responseBuffer), "\n")
	for _, obj := range parsedObjects {
		modelResponseResult := gjson.Get(obj, "result.response.modelResponse")
		if modelResponseResult.Type == gjson.Null {
			token := ""
			tokenResult := gjson.Get(obj, "result.response.token")
			if tokenResult.Type == gjson.String {
				token = tokenResult.String()
			} else {
				continue
			}

			isThinkingResult := gjson.Get(obj, "result.response.isThinking")
			if isThinkingResult.Type == gjson.True {
				think = think + token
			} else if isThinkingResult.Type == gjson.False {
				body = body + token
			}
		} else {
			messageResult := modelResponseResult.Get("message")
			if messageResult.Type == gjson.String {
				body = messageResult.String()
			}
			thinkingTraceResult := modelResponseResult.Get("thinkingTrace")
			if thinkingTraceResult.Type == gjson.String {
				think = thinkingTraceResult.String()
			}
		}
	}

	return &AdapterResponse{
		Content:          body,
		ReasoningContent: think,
		ToolCalls:        toolCalls,
		Done:             done,
	}, nil
}
