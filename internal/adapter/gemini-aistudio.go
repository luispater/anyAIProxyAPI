package adapter

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"regexp"
	"strings"
)

func init() {
	Adapters["gemini-aistudio"] = &GeminiAIStudioAdapter{}
}

type GeminiAIStudioAdapter struct {
}

func (g *GeminiAIStudioAdapter) HandleResponse(responseBuffer []byte, done bool) (*AdapterResponse, error) {
	pattern := `\[\[\[null,(.*?)]],"model"]`
	re := regexp.MustCompile(pattern)

	think := ""
	body := ""
	toolCalls := ""
	arrToolCalls := make([]string, 0)
	input := string(responseBuffer)
	matches := re.FindAllString(input, -1)
	for _, match := range matches {
		value := gjson.Get(match, "0.0")
		if value.IsArray() {
			arr := value.Array()
			if len(arr) == 2 {
				body = body + arr[1].String()
			} else if len(arr) == 11 && arr[1].Type == gjson.Null && arr[10].Type == gjson.JSON {
				if !arr[10].IsArray() {
					continue
				}
				arrayToolCalls := arr[10].Array()
				funcName := arrayToolCalls[0].String()
				argumentsStr := arrayToolCalls[1].String()
				params := g.parseToolCallParams(argumentsStr)

				toolCallsTemplate := `{"id":"","index":0,"type":"function","function":{"name":"","arguments":""}}`
				tcs, _ := sjson.Set(toolCallsTemplate, "function.name", funcName)
				tcs, _ = sjson.Set(tcs, "function.arguments", params)
				arrToolCalls = append(arrToolCalls, tcs)
			} else if len(arr) > 2 {
				think = think + arr[1].String()
			}
		}
	}
	if len(arrToolCalls) > 0 {
		toolCalls = "[" + strings.Join(arrToolCalls, ",") + "]"
	}

	result := &AdapterResponse{
		Content:          body,
		ReasoningContent: think,
		ToolCalls:        toolCalls,
		Done:             done,
	}
	return result, nil
}

func (g *GeminiAIStudioAdapter) parseToolCallParams(argumentsStr string) string {
	arguments := gjson.Get(argumentsStr, "0")
	if !arguments.IsArray() {
		return ""
	}
	funcParams := `{}`
	args := arguments.Array()
	for i := 0; i < len(args); i++ {
		if args[i].IsArray() {
			arg := args[i].String()
			paramName := gjson.Get(arg, "0")
			paramValue := gjson.Get(arg, "1")
			if paramValue.IsArray() {
				v := paramValue.Array()
				if len(v) == 1 { // null
					funcParams, _ = sjson.Set(funcParams, paramName.String(), nil)
				} else if len(v) == 2 { // number and integer
					funcParams, _ = sjson.Set(funcParams, paramName.String(), v[1].Value())
				} else if len(v) == 3 { // string
					funcParams, _ = sjson.Set(funcParams, paramName.String(), v[2].String())
				} else if len(v) == 4 { // Boolean
					funcParams, _ = sjson.Set(funcParams, paramName.String(), v[3].Int() == 1)
				} else if len(v) == 5 { // object
					result := g.parseToolCallParams(v[4].Raw)
					if result == "" {
						funcParams, _ = sjson.Set(funcParams, paramName.String(), nil)
					} else {
						funcParams, _ = sjson.SetRaw(funcParams, paramName.String(), result)
					}
				}
			}
		}
	}
	return funcParams
}
