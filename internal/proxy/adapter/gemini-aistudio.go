package adapter

import (
	"bytes"
	"compress/gzip"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/utils"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	Adapters["gemini-aistudio"] = &GeminiAIStudioAdapter{}
}

type GeminiAIStudioAdapter struct {
}

func (g *GeminiAIStudioAdapter) ShouldRecord(buffer []byte) bool {
	return bytes.Contains(buffer, []byte("GenerateContent"))
}

func (g *GeminiAIStudioAdapter) HandleResponse(responseBuffer chan []byte, disconnect chan bool, sniffing *bool, queue *utils.Queue[*model.ProxyResponse]) {
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
}

func (g *GeminiAIStudioAdapter) decompressGzip(dataBuffer []byte, done bool) (*model.ProxyResponse, error) {
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
			result = append(result, buf[:n-1]...)
		}
	}
	_ = gzReader.Close()

	pattern := `\[\[\[null,(.*?)]],"model"]`
	re := regexp.MustCompile(pattern)

	think := ""
	body := ""
	toolCalls := ""
	arrToolCalls := make([]string, 0)
	input := string(result)
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

	return &model.ProxyResponse{
		Context:          body,
		ReasoningContent: think,
		ToolCalls:        toolCalls,
		Done:             done,
	}, nil
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
