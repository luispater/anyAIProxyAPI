package adapter

import (
	"bytes"
	"compress/gzip"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"strconv"
	"strings"
)

func init() {
	Adapters["grok"] = &GrokAdapter{}
}

type GrokAdapter struct {
}

func (g *GrokAdapter) ShouldRecord(buffer []byte) bool {
	return bytes.Contains(buffer, []byte("rest/app-chat/conversations/new")) || (bytes.Contains(buffer, []byte("rest/app-chat/conversations/")) && bytes.Contains(buffer, []byte("/responses")))
}

func (g *GrokAdapter) HandleResponse(responseBuffer chan []byte, disconnect chan bool, sniffing *bool, queue *utils.Queue[*model.ProxyResponse]) {
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
						if bytes.Contains(responseHeader, []byte("Content-Encoding: gzip")) {
							dataBuffer = append(dataBuffer, chunkedData...)
							result, errDecompressGzip := g.decompressGzip(dataBuffer, false)
							if errDecompressGzip == nil {
								if *sniffing {
									queue.Enqueue(result)
								}
							}
							log.Debug(string(responseHeader))
							log.Debug(result, errDecompressGzip)
						}
					}
				}
			}
		case <-disconnect:
			break outLoop
		}
	}

	result, errDecompressGzip := g.decompressGzip(dataBuffer, true)
	if errDecompressGzip == nil {
		if *sniffing {
			queue.Enqueue(result)
		}
	}
	log.Debug(result, errDecompressGzip)
}

func (g *GrokAdapter) decompressGzip(dataBuffer []byte, done bool) (*model.ProxyResponse, error) {
	buffer := bytes.NewBuffer(dataBuffer)
	gzReader, errReader := gzip.NewReader(buffer)
	if errReader != nil {
		return nil, errReader
	}
	result := make([]byte, 0)
	for {
		buf := make([]byte, 4096)
		n, errRead := gzReader.Read(buf)
		if errRead != nil {
			// if errRead == io.EOF {
			// 	log.Infof("EOF")
			// } else {
			// 	log.Errorf("Unknow error: %v", errRead)
			// }
			break
		}
		if n > 0 {
			result = append(result, buf[:n]...)
		}
	}
	_ = gzReader.Close()

	think := ""
	body := ""
	toolCalls := ""

	parsedObjects := strings.Split(string(result), "\n")
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

	return &model.ProxyResponse{
		Content:          body,
		ReasoningContent: think,
		ToolCalls:        toolCalls,
		Done:             done,
	}, nil
}

func (g *GrokAdapter) extractJsonStrings(str string) []string {
	var objects []string
	var start int
	braceCount := 0
	inString := false
	escape := false

	for i, char := range str {
		if char == '\\' && inString {
			escape = !escape
			continue
		}
		if char == '"' && !escape {
			inString = !inString
		}
		escape = false

		if !inString {
			if char == '{' {
				if braceCount == 0 {
					start = i
				}
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					jsonStr := str[start : i+1]
					objects = append(objects, jsonStr)
				}
			}
		}
	}

	return objects
}
