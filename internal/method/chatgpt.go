package method

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

func (m *Method) ChooseChatGPTModelByName(name, modelSelector, more string) error {
	modelContainers, err := m.GetElements(modelSelector) // This now returns []*cdp.Node
	if err != nil {
		return err
	}
	var moreNode *cdp.Node
	for i := 0; i < len(modelContainers); i++ {
		containerNode := modelContainers[i]

		var innerText string
		textCtx, cancel := context.WithTimeout(m.page.GetContext(), 1000*time.Millisecond) // Shorter timeout for text retrieval
		_ = cancel
		err = chromedp.Run(textCtx,
			chromedp.InnerHTML(".flex.items-center", &innerText, chromedp.ByQueryAll, chromedp.FromNode(containerNode)),
		)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`<[^>]+>.*?</[^>]+>`)
		innerText = re.ReplaceAllString(innerText, "")

		if strings.ToLower(innerText) == strings.ToLower(name) {
			clickCtx, cancelClick := context.WithTimeout(m.page.GetContext(), 2500*time.Millisecond)
			_ = cancelClick
			log.Debugf("Found model '%s', attempting to click node (Name: %s, ID: %d)", name, containerNode.NodeName, containerNode.NodeID)
			return chromedp.Run(clickCtx,
				chromedp.MouseClickNode(containerNode),
			)
		} else if innerText == more {
			moreNode = containerNode
		}
	}

	if moreNode != nil {
		clickCtx, cancelClick := context.WithTimeout(m.page.GetContext(), 2500*time.Millisecond)
		_ = cancelClick
		err = chromedp.Run(clickCtx,
			chromedp.MouseClickNode(moreNode),
		)
		if err != nil {
			return err
		}
	}

	modelContainers, err = m.GetElements(modelSelector) // This now returns []*cdp.Node
	if err != nil {
		return err
	}
	for i := 0; i < len(modelContainers); i++ {
		containerNode := modelContainers[i]

		var innerText string
		textCtx, cancel := context.WithTimeout(m.page.GetContext(), 1000*time.Millisecond) // Shorter timeout for text retrieval
		_ = cancel
		err = chromedp.Run(textCtx,
			chromedp.InnerHTML(".flex.items-center", &innerText, chromedp.ByQueryAll, chromedp.FromNode(containerNode)),
		)
		if err != nil {
			return err
		}

		re := regexp.MustCompile(`<[^>]+>.*?</[^>]+>`)
		innerText = re.ReplaceAllString(innerText, "")
		// log.Debug(innerText)

		if strings.ToLower(innerText) == strings.ToLower(name) {
			clickCtx, cancelClick := context.WithTimeout(m.page.GetContext(), 2500*time.Millisecond)
			_ = cancelClick
			log.Debugf("Found model '%s', attempting to click node (Name: %s, ID: %d)", name, containerNode.NodeName, containerNode.NodeID)
			return chromedp.Run(clickCtx,
				chromedp.MouseClickNode(containerNode),
			)
		}
	}

	return fmt.Errorf("model '%s' not found after checking all categories", name)
}
