package method

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func (m *Method) Value(elementSelector string, timeout float64) (string, error) {
	opCtx := m.page.GetContext()
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.page.GetContext(), time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	// First, ensure the element exists to provide a better error message.
	var nodes []*cdp.Node
	err := chromedp.Run(opCtx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery, chromedp.AtLeast(0)),
	)
	if err != nil {
		return "", fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		_ = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		return "", fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	var value string
	if err := chromedp.Run(opCtx, chromedp.Value(elementSelector, &value, chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("error getting value from element '%s': %v", elementSelector, err)
	}
	return value, nil
}
