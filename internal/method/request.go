package method

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"strings"
)

func (m *Method) Temperature(requestJson string) (bool, string, error) {
	temperatureResult := gjson.Get(requestJson, "temperature")
	if temperatureResult.Type == gjson.Number {
		return true, temperatureResult.String(), nil
	}
	return false, "", nil
}

func (m *Method) TopP(requestJson string) (bool, string, error) {
	topPResult := gjson.Get(requestJson, "top_p")
	if topPResult.Type == gjson.Number {
		return true, topPResult.String(), nil
	}
	return false, "", nil
}

func (m *Method) StopSequence(requestJson string) (bool, string, error) {
	stopResult := gjson.Get(requestJson, "stop")
	if stopResult.Type == gjson.String {
		return true, stopResult.String(), nil
	}
	return false, "", nil
}

func (m *Method) PromptCount(requestJson string) (systemPromptCount int, assistantPromptCount int, userPromptCount int, toolPromptCount int, error error) {
	systemPromptCount = 0
	userPromptCount = 0
	toolPromptCount = 0
	assistantPromptCount = 0

	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		error = fmt.Errorf("messages not define")
		return
	}
	messages := messagesResult.Array()
	for _, msg := range messages {
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			error = fmt.Errorf("role is not a string")
			return
		}
		role := roleResult.String()
		if role == "system" {
			systemPromptCount++
		} else if role == "user" {
			userPromptCount++
		} else if role == "tool" {
			toolPromptCount++
		} else if role == "assistant" {
			assistantPromptCount++
		}
	}
	return
}

func (m *Method) SystemPrompt(requestJson string) (bool, string, error) {
	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		return false, "", fmt.Errorf("messages not define")
	}
	messages := messagesResult.Array()
	if len(messages) > 0 {
		msg := messages[0]
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			return false, "", fmt.Errorf("role is not a string")
		}
		role := roleResult.String()
		if role == "system" {
			contentResult := gjson.Get(msg.Raw, "content")
			if contentResult.Type == gjson.String {
				return true, contentResult.String(), nil
			} else if contentResult.IsObject() {
				textResult := contentResult.Get("text")
				if textResult.Type != gjson.String {
					return false, "", fmt.Errorf("text is not a string")
				}
				return true, textResult.String(), nil
			} else {
				return false, "", fmt.Errorf("context is not a string or object")
			}
		}
	}
	return false, "", nil
}

func (m *Method) UserPrompt(requestJson string) (bool, string, error) {
	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		log.Error("messages not define")
		return false, "", fmt.Errorf("messages not define")
	}
	messages := messagesResult.Array()

	if len(messages) > 0 {
		msg := messages[len(messages)-1]
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			log.Error("role is not a string")
			return false, "", fmt.Errorf("role is not a string")
		}
		role := roleResult.String()
		if role == "user" {
			contentResult := gjson.Get(msg.Raw, "content")
			if contentResult.Type == gjson.String {
				return true, contentResult.String(), nil
			} else if contentResult.IsArray() {
				contents := contentResult.Array()
				for i := 0; i < len(contents); i++ {
					contentType := contents[i].Get("type").String()
					if contentType == "text" {
						contentTextResult := contents[i].Get("text")
						if contentTextResult.Type != gjson.String {
							log.Error("text is not a string")
							return false, "", fmt.Errorf("text is not a string")
						}
						return true, contentTextResult.String(), nil
					}
				}
			} else {
				log.Error("context is not a string or object")
				return false, "", fmt.Errorf("context is not a string or object")
			}
		} else {
			return false, "", fmt.Errorf("role is not user")
		}
	}
	return false, "", fmt.Errorf("messages is emtpy")
}

func (m *Method) BuildPrompt(requestJson string, includeSystem bool) (string, error) {
	userPromptCount := 0
	assistantPromptCount := 0
	systemPromptCount := 0

	prompts := make([]string, 0)

	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		return "", fmt.Errorf("messages not define")
	}
	messages := messagesResult.Array()
	for _, msg := range messages {
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			return "", fmt.Errorf("role is not a string")
		}
		role := roleResult.String()
		if role == "user" {
			userPromptCount++
		} else if role == "assistant" {
			assistantPromptCount++
		} else if role == "system" && includeSystem {
			systemPromptCount++
		}
	}

	if assistantPromptCount > 0 || systemPromptCount > 0 {
		for _, msg := range messages {
			msgText := ""

			roleResult := gjson.Get(msg.Raw, "role")
			if roleResult.Type != gjson.String {
				log.Error("role is not a string")
				continue
			}

			role := roleResult.String()
			if role == "user" {
				msgText = "user:\n"
			} else if role == "assistant" {
				msgText = "model:\n"
			} else {
				continue
			}

			contentResult := gjson.Get(msg.Raw, "content")

			if contentResult.Type == gjson.String {
				msgText = msgText + contentResult.String()
			} else if contentResult.IsArray() {
				contents := contentResult.Array()
				for i := 0; i < len(contents); i++ {
					contentType := contents[i].Get("type").String()
					if contentType == "text" {
						contentTextResult := contents[i].Get("text")
						if contentTextResult.Type != gjson.String {
							log.Error("text is not a string")
							return "", fmt.Errorf("text is not a string")
						}
						msgText = msgText + contentTextResult.String()
					}
				}
			} else {
				log.Error("context is not a string or array")
				return "", fmt.Errorf("context is not a string or array")
			}

			prompts = append(prompts, strings.TrimSpace(msgText))
		}
	} else {
		for _, msg := range messages {
			msgText := ""

			roleResult := gjson.Get(msg.Raw, "role")
			if roleResult.Type != gjson.String {
				log.Error("role is not a string")
				continue
			}

			role := roleResult.String()
			if role != "user" {
				continue
			}

			contentResult := gjson.Get(msg.Raw, "content")

			if contentResult.Type == gjson.String {
				msgText = contentResult.String()
			} else if contentResult.IsArray() {
				contents := contentResult.Array()
				for i := 0; i < len(contents); i++ {
					contentType := contents[i].Get("type").String()
					if contentType == "text" {
						contentTextResult := contents[i].Get("text")
						if contentTextResult.Type != gjson.String {
							log.Error("text is not a string")
							return "", fmt.Errorf("text is not a string")
						}
						msgText = contentTextResult.String()
					}
				}
			} else {
				log.Error("context is not a string or array")
				return "", fmt.Errorf("context is not a string or array")
			}

			prompts = append(prompts, strings.TrimSpace(msgText))
		}
	}

	if includeSystem {
		systemPrompt := make([]string, 0)
		for _, msg := range messages {
			roleResult := gjson.Get(msg.Raw, "role")
			if roleResult.Type != gjson.String {
				log.Error("role is not a string")
				continue
			}

			role := roleResult.String()
			if role != "system" {
				continue
			}

			contentResult := gjson.Get(msg.Raw, "content")

			if contentResult.Type == gjson.String {
				systemPrompt = append(systemPrompt, contentResult.String())
			} else if contentResult.IsArray() {
				contents := contentResult.Array()
				for i := 0; i < len(contents); i++ {
					contentType := contents[i].Get("type").String()
					if contentType == "text" {
						contentTextResult := contents[i].Get("text")
						if contentTextResult.Type != gjson.String {
							log.Error("text is not a string")
							return "", fmt.Errorf("text is not a string")
						}
						systemPrompt = append(systemPrompt, contentTextResult.String())
					}
				}
			} else {
				log.Error("context is not a string or array")
				return "", fmt.Errorf("context is not a string or array")
			}
		}
		if len(systemPrompt) > 0 {
			systemPromptText := "system:\n" + strings.Join(systemPrompt, "\n")
			prompts = append([]string{systemPromptText}, prompts...)
		}
	}

	if len(prompts) > 0 {
		return strings.Join(prompts, "\n\n"), nil
	}
	return "", fmt.Errorf("message is empty")
}

func (m *Method) ImagePrompt(requestJson string) (bool, []string, error) {
	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		log.Error("messages not define")
		return false, nil, fmt.Errorf("messages not define")
	}
	messages := messagesResult.Array()
	arrayImageURL := make([]string, 0)
	if len(messages) > 0 {
		msg := messages[len(messages)-1]
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			log.Error("role is not a string")
			return false, nil, fmt.Errorf("role is not a string")
		}
		role := roleResult.String()
		if role == "user" {
			contentResult := gjson.Get(msg.Raw, "content")
			if contentResult.Type == gjson.String {
				return false, nil, fmt.Errorf("content has not any image_url")
			} else if contentResult.IsArray() {
				contents := contentResult.Array()
				for i := 0; i < len(contents); i++ {
					contentType := contents[i].Get("type").String()
					if contentType == "image_url" {
						contentImageURLResult := contents[i].Get("image_url.url")
						if contentImageURLResult.Type != gjson.String {
							log.Error("image_url.url is not a string")
							return false, nil, fmt.Errorf("image_url.url is not a string")
						}
						imageUrl := contentImageURLResult.String()
						if imageUrl != "" {
							arrayImageURL = append(arrayImageURL, imageUrl)
						}
					}
				}
			} else {
				log.Error("context is not a string or array")
				return false, nil, fmt.Errorf("context is not a string or array")
			}
		} else {
			return false, nil, fmt.Errorf("role is not user")
		}

		if len(arrayImageURL) > 0 {
			return true, arrayImageURL, nil
		}
	}
	return false, nil, fmt.Errorf("messages is emtpy")
}

func (m *Method) ToolPrompt(requestJson string) (bool, string, error) {
	messagesResult := gjson.Get(requestJson, "messages")
	if !messagesResult.IsArray() {
		log.Info("messages not define")
		return false, "", fmt.Errorf("messages not define")
	}
	messages := messagesResult.Array()
	if len(messages) > 0 {
		msg := messages[len(messages)-1]
		roleResult := gjson.Get(msg.Raw, "role")
		if roleResult.Type != gjson.String {
			log.Info("role is not a string")
			return false, "", fmt.Errorf("role is not a string")
		}
		role := roleResult.String()
		if role == "tool" {
			contentResult := gjson.Get(msg.Raw, "content")
			if contentResult.Type == gjson.String {
				return true, contentResult.String(), nil
			} else {
				log.Info("context is not a string")
				return false, "", fmt.Errorf("context is not a string")
			}
		}
	}
	return false, "", nil
}

func (m *Method) MaxTokens(requestJson string) (bool, string, error) {
	maxTokensResult := gjson.Get(requestJson, "max_tokens")
	if maxTokensResult.Type == gjson.Number {
		return true, maxTokensResult.String(), nil
	}
	return false, "", nil
}

func (m *Method) Model(requestJson string) (bool, string) {
	modelResult := gjson.Get(requestJson, "model")
	if modelResult.Type == gjson.String {
		return true, modelResult.String()
	}
	return false, ""
}

func (m *Method) Tools(requestJson string) (bool, string, error) {
	toolsResult := gjson.Get(requestJson, "tools")
	if toolsResult.Type == gjson.Null {
		return false, "", nil
	}
	if toolsResult.IsArray() {
		arrayTools := make([]string, 0)
		tools := toolsResult.Array()
		for _, tool := range tools {
			functionResult := tool.Get("function")
			if functionResult.IsObject() {
				arrayTools = append(arrayTools, functionResult.Raw)
			}
		}
		return true, "[" + strings.Join(arrayTools, ",") + "]", nil
	}
	return true, "[]", nil
}
