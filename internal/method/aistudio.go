package method

import (
	"context"
	"fmt"
	"time"

	"strconv"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) ChooseModelByName(name, modelNameSelector string, modelCategoryContainerSelector string) error {
	modelCategoryContainers, err := m.GetElements(modelCategoryContainerSelector) // This now returns []*cdp.Node
	if err != nil {
		return err
	}
	for i := 0; i < len(modelCategoryContainers); i++ {
		containerNode := modelCategoryContainers[i]
		clickCtx, cancelClick := context.WithTimeout(m.page.GetContext(), 2500*time.Millisecond)
		err = chromedp.Run(clickCtx,
			chromedp.MouseClickNode(containerNode),
		)
		cancelClick() // Always call cancel
		log.Info(err)
		if err == nil {
			// After clicking the container, model names might appear or update
			modelNameElements, errGetElements := m.GetElements(modelNameSelector) // This now returns []*cdp.Node
			if errGetElements != nil {
				log.Warnf("Error getting model name elements after clicking container: %v", errGetElements)
				continue
			}
			for j := 0; j < len(modelNameElements); j++ {
				modelNode := modelNameElements[j]
				var innerText string
				// It's better to use a new context for each Run if timeouts are critical per operation
				textCtx, cancel := context.WithTimeout(m.page.GetContext(), 1000*time.Millisecond) // Shorter timeout for text retrieval
				_ = cancel
				errText := chromedp.Run(textCtx,
					chromedp.TextContent([]cdp.NodeID{modelNode.NodeID}, &innerText, chromedp.ByNodeID),
				)
				// textCancel()

				if errText != nil {
					log.Warnf("Error getting inner text for model node (Name: %s, ID: %d): %v", modelNode.NodeName, modelNode.NodeID, errText)
					continue
				}

				if strings.Contains(innerText, name) {
					finalClickCtx, cancelFinalClick := context.WithTimeout(m.page.GetContext(), 2500*time.Millisecond)
					defer cancelFinalClick()
					log.Debugf("Found model '%s', attempting to click node (Name: %s, ID: %d)", name, modelNode.NodeName, modelNode.NodeID)
					return chromedp.Run(finalClickCtx,
						chromedp.MouseClickNode(modelNode),
					)
				}
			}
		} else {
			log.Warnf("Error clicking model category container node (Name: %s, ID: %d): %v", containerNode.NodeName, containerNode.NodeID, err)
		}
	}
	return fmt.Errorf("model '%s' not found after checking all categories", name)
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
