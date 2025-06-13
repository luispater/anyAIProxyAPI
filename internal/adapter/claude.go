package adapter

import (
	"fmt"
	"github.com/tidwall/gjson"
	"regexp"
)

func init() {
	Adapters["claude"] = &ClaudeAdapter{}
}

type ClaudeAdapter struct {
}

func (g *ClaudeAdapter) HandleResponse(responseBuffer []byte, done bool) (*AdapterResponse, error) {
	content := ""
	reasoningContent := ""
	pattern := `data:(.*?)\n\n`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(string(responseBuffer), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no match data")
	}
	thinkStatus := false
	for i := 0; i < len(matches); i++ {
		match := matches[i]
		if len(match) == 2 {
			c, d := g.getDataContent(match[1], &thinkStatus)
			if !d {
				if thinkStatus {
					reasoningContent = reasoningContent + c
				} else {
					content = content + c
				}
			} else {
				done = true
				break
			}
		}
	}

	return &AdapterResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		ToolCalls:        "",
		Done:             done,
	}, nil

}

func (g *ClaudeAdapter) getDataContent(jsonData string, thinkStatus *bool) (string, bool) {
	typeResult := gjson.Get(jsonData, "type")
	if typeResult.Type != gjson.String {
		return "", false
	}
	content := ""
	switch typeResult.String() {
	case "content_block_start":
		blockTypeResult := gjson.Get(jsonData, "content_block.type")
		if blockTypeResult.Type == gjson.String {
			if blockTypeResult.String() == "thinking" {
				*thinkStatus = true
				thinkingResult := gjson.Get(jsonData, "content_block.thinking")
				if thinkingResult.Type == gjson.String {
					content = thinkingResult.String()
				}
			} else if blockTypeResult.String() == "text" {
				*thinkStatus = false
				textResult := gjson.Get(jsonData, "content_block.text")
				if textResult.Type == gjson.String {
					content = textResult.String()
				}
			}
		}
	case "content_block_delta":
		blockDeltaTypeResult := gjson.Get(jsonData, "delta.type")
		if blockDeltaTypeResult.Type == gjson.String {
			if blockDeltaTypeResult.String() == "thinking_delta" {
				textResult := gjson.Get(jsonData, "delta.thinking")
				if textResult.Type == gjson.String {
					content = textResult.String()
				}
			} else if blockDeltaTypeResult.String() == "text_delta" {
				textResult := gjson.Get(jsonData, "delta.text")
				if textResult.Type == gjson.String {
					content = textResult.String()
				}
			}
		}
	case "content_block_stop":
	case "message_stop":
		return "", true
	}
	return content, false
}
