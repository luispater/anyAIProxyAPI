package adapter

import (
	"fmt"
	"github.com/tidwall/gjson"
	"regexp"
	"strings"
)

func init() {
	Adapters["chatgpt"] = &ChatGPTAdapter{}
}

type ChatGPTAdapter struct {
}

func (g *ChatGPTAdapter) HandleResponse(responseBuffer []byte, done bool) (*AdapterResponse, error) {
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

func (g *ChatGPTAdapter) getDataContent(jsonData string, thinkStatus *bool) (string, bool) {
	if jsonData == "[DONE]" {
		return "", true
	}
	content := ""

	pResult := gjson.Get(jsonData, "p")
	if pResult.Type == gjson.String {
		if pResult.String() == "/message/content/thoughts/0/summary" {
			return "", false
		} else if pResult.String() == "/message/content/thoughts" {
			*thinkStatus = true
		} else if strings.HasPrefix(pResult.String(), "/message/content/parts") {
			*thinkStatus = false
		}
	}

	operationResult := gjson.Get(jsonData, "o")
	if operationResult.Type == gjson.Null {
		vResult := gjson.Get(jsonData, "v")
		if vResult.Type == gjson.String {
			content = content + gjson.Get(jsonData, "v").String()
		}
	} else if operationResult.Type == gjson.String {
		operation := operationResult.String()
		if operation == "add" {
			part0Result := gjson.Get(jsonData, "v.message.content.parts.0")
			if part0Result.Type == gjson.String {
				content = content + part0Result.String()
			}
		} else if operation == "patch" {
			patchesResult := gjson.Get(jsonData, "v")
			if patchesResult.IsArray() {
				patches := patchesResult.Array()
				for i := 0; i < len(patches); i++ {
					patch := patches[i]
					ppResult := patch.Get("p")
					if ppResult.Type == gjson.String {
						if ppResult.String() == "/message/content/thoughts/0/summary" {
							continue
						}
					}
					opResult := patch.Get("o")
					if opResult.Type == gjson.String {
						op := opResult.String()
						if op == "append" {
							valueResult := patch.Get("v")
							if valueResult.Type == gjson.String {
								content = content + valueResult.String()
							}
						}
					}
				}
			}
		} else if operation == "append" {
			vResult := gjson.Get(jsonData, "v")
			if vResult.Type == gjson.String {
				content = content + gjson.Get(jsonData, "v").String()
			}
		}
	}
	return content, false
}
