package method

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

func (m *Method) ChooseModelByName(name, modelNameSelector string, modelCategoryContainerSelector string) error {
	modelCategoryContainers, err := m.GetElements(modelCategoryContainerSelector)
	if err != nil {
		return err
	}
	for i := 0; i < len(modelCategoryContainers); i++ {
		err = modelCategoryContainers[i].Click(playwright.LocatorClickOptions{Timeout: playwright.Float(2500)})
		if err == nil {
			modelNameElements, errGetElements := m.GetElements(modelNameSelector)
			if errGetElements != nil {
				continue
			}
			for j := 0; j < len(modelNameElements); j++ {
				innerText, _ := modelNameElements[j].InnerText()
				if strings.Contains(innerText, name) {
					return modelNameElements[j].Click(playwright.LocatorClickOptions{Timeout: playwright.Float(2500)})
				}
			}
		}
	}
	return fmt.Errorf("model not found")
}

func (m *Method) AIStudioTokens(systemTokens, inputTokens, endTokens string) (uint64, uint64, uint64, error) {
	systemTokens = strings.TrimSpace(systemTokens)
	inputTokens = strings.TrimSpace(inputTokens)
	endTokens = strings.TrimSpace(endTokens)

	systemTokens = strings.ReplaceAll(systemTokens, ",", "")
	inputTokens = strings.ReplaceAll(inputTokens, ",", "")
	endTokens = strings.ReplaceAll(endTokens, ",", "")

	systemTokens = strings.Split(systemTokens, "/")[0]
	inputTokens = strings.Split(inputTokens, "/")[0]
	endTokens = strings.Split(endTokens, "/")[0]

	systemTokens = strings.TrimSpace(systemTokens)
	inputTokens = strings.TrimSpace(inputTokens)
	endTokens = strings.TrimSpace(endTokens)

	totalTokens, err := strconv.ParseUint(endTokens, 10, 64)
	if err != nil {
		log.Error("convert total tokens failed: %v", err)
		return 0, 0, 0, err
	}

	promptTokens, err := strconv.ParseUint(inputTokens, 10, 64)
	if err != nil {
		log.Error("convert prompt tokens failed: %v", err)
		return 0, 0, 0, err
	}

	completionTokens := totalTokens - promptTokens

	return promptTokens, completionTokens, totalTokens, nil
}
