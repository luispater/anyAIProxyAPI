package adapter

import (
	"bytes"
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"regexp"
	"strconv"
)

func init() {
	Adapters["chatgpt"] = &ChatGPTAdapter{}
}

type ChatGPTAdapter struct {
}

func (g *ChatGPTAdapter) ShouldRecord(buffer []byte) bool {
	return bytes.Contains(buffer, []byte("/backend-api/conversation ")) // keep the last space
}

func (g *ChatGPTAdapter) HandleResponse(responseBuffer chan []byte, disconnect chan bool, sniffing *bool, queue *utils.Queue[*model.ProxyResponse]) {
	hasAllHeader := false
	responseHeader := make([]byte, 0)
	responseBody := make([]byte, 0)
	var transferEncoding string
	dataBuffer := make([]byte, 0)
outLoop:
	for {
		select {
		case data := <-responseBuffer:
			if !hasAllHeader {
				sIdx := bytes.Index(data, []byte("\r\n\r\n"))
				if sIdx != -1 {
					responseHeader = append(responseHeader, data[0:sIdx]...)
					responseBody = append(responseBody, data[sIdx+4:]...)

					hasAllHeader = true
				} else {
					responseHeader = append(responseHeader, data...)
				}
			} else {
				if bytes.Contains(responseHeader, []byte("Transfer-Encoding: chunked")) {
					transferEncoding = "chunked"
				}
				responseBody = append(responseBody, data...)

				if transferEncoding == "chunked" {
					for {
						lengthCrlfIdx := bytes.Index(responseBody, []byte("\r\n"))
						if lengthCrlfIdx == -1 {
							break
						}
						hexLength := responseBody[:lengthCrlfIdx]
						length, errParseInt := strconv.ParseInt(string(hexLength), 16, 64)
						if errParseInt != nil {
							log.Warnf("Parsing chunked length failed: %v", errParseInt)
							hasAllHeader = false
							responseHeader = make([]byte, 0)
							responseBody = make([]byte, 0)
							transferEncoding = ""
							break
						}
						if length == 0 {
							if bytes.Contains(responseBody[:5], []byte("0\r\n\r\n")) {
								hasAllHeader = false
								responseHeader = make([]byte, 0)
								responseBody = make([]byte, 0)
								transferEncoding = ""
								break
							}
						}
						if int(length)+2 > len(responseBody) {
							break
						}

						chunkedData := responseBody[lengthCrlfIdx+2 : lengthCrlfIdx+2+int(length)]
						if lengthCrlfIdx+2+int(length)+2 > len(responseBody) {
							continue
						}
						responseBody = responseBody[lengthCrlfIdx+2+int(length)+2:]
						dataBuffer = append(dataBuffer, chunkedData...)

						result, err := g.createResponse(dataBuffer, false)
						if err == nil {
							if *sniffing {
								queue.Enqueue(result)
							}
						}

					}
				}
			}
		case <-disconnect:
			break outLoop
		}
	}

	result, err := g.createResponse(dataBuffer, true)
	if err == nil {
		if *sniffing {
			queue.Enqueue(result)
		}
	}

}

func (g *ChatGPTAdapter) createResponse(dataBuffer []byte, done bool) (*model.ProxyResponse, error) {
	content := ""

	pattern := `data:(.*?)\n\n`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(string(dataBuffer), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no match data")
	}

	for i := 0; i < len(matches); i++ {
		match := matches[i]
		// log.Infof("Buffer 1: %s", match[0])
		// log.Infof("Buffer 2: %s", match[1])
		if len(match) == 2 {
			// log.Info(match[1])
			c, d := g.getDataContent(match[1])
			if !d {
				content = content + c
			} else {
				done = true
				break
			}
		}
	}

	return &model.ProxyResponse{
		Content:          content,
		ReasoningContent: "",
		ToolCalls:        "",
		Done:             done,
	}, nil
}

func (g *ChatGPTAdapter) getDataContent(jsonData string) (string, bool) {
	if jsonData == "[DONE]" {
		return "", true
	}
	result := ""
	operationResult := gjson.Get(jsonData, "o")
	if operationResult.Type == gjson.Null {
		vResult := gjson.Get(jsonData, "v")
		if vResult.Type == gjson.String {
			result = result + gjson.Get(jsonData, "v").String()
		}
	} else if operationResult.Type == gjson.String {
		operation := operationResult.String()
		if operation == "add" {
			part0Result := gjson.Get(jsonData, "v.message.content.parts.0")
			if part0Result.Type == gjson.String {
				result = result + part0Result.String()
			}
		} else if operation == "patch" {
			patchesResult := gjson.Get(jsonData, "v")
			if patchesResult.IsArray() {
				patches := patchesResult.Array()
				for i := 0; i < len(patches); i++ {
					patch := patches[i]
					opResult := patch.Get("o")
					if opResult.Type == gjson.String {
						op := opResult.String()
						if op == "append" {
							valueResult := patch.Get("v")
							if valueResult.Type == gjson.String {
								result = result + valueResult.String()
							}
						}
					}
				}
			}
		} else if operation == "append" {
			vResult := gjson.Get(jsonData, "v")
			if vResult.Type == gjson.String {
				result = result + gjson.Get(jsonData, "v").String()
			}
		}
	}
	return result, false
}
