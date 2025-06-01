package method

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

var lastClickTime time.Time

const clickInterval = 200 * time.Millisecond

func (m *Method) Click(elementSelector string, timeout float64) error {
	if time.Since(lastClickTime) < clickInterval {
		log.Debugf("Click too fast, wait for 200ms")
		time.Sleep(clickInterval)
	}
	lastClickTime = time.Now()
	log.Debugf("Attempting to find and click element with selector: %s", elementSelector)

	opCtx := m.page.GetContext()
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.page.GetContext(), time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	err := chromedp.Run(opCtx,
		// First, wait for the element to be visible. This replaces the IsVisible check.
		chromedp.WaitVisible(elementSelector, chromedp.ByQuery),
		// Then, click the element.
		chromedp.Click(elementSelector, chromedp.ByQuery),
	)

	if err != nil {
		var currentURL string
		// Best effort to get URL for error context
		_ = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		return fmt.Errorf("error clicking element '%s' on page %s: %v", elementSelector, currentURL, err)
	}

	log.Debugf("Successfully clicked element '%s'.", elementSelector)
	return nil
}

func (m *Method) MouseClick(x, y float64) error {
	if time.Since(lastClickTime) < clickInterval {
		log.Debugf("Click too fast, wait for 200ms")
		time.Sleep(clickInterval)
	}

	return chromedp.Run(m.page.GetContext(), chromedp.MouseClickXY(x, y))
}
