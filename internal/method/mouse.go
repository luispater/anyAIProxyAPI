package method

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

var lastClickTime time.Time

func (m *Method) Click(elementSelector string, timeout float64) error {
	if time.Since(lastClickTime) < 400*time.Millisecond {
		log.Debugf("Click too fast, wait for 1s")
		time.Sleep(400 * time.Millisecond)
	}
	lastClickTime = time.Now()
	log.Debugf("Attempting to find and click element with selector: %s", elementSelector)

	opCtx := m.pageCtx
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.pageCtx, time.Duration(timeout*float64(time.Millisecond)))
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
		_ = chromedp.Run(m.pageCtx, chromedp.Location(&currentURL))
		return fmt.Errorf("error clicking element '%s' on page %s: %v", elementSelector, currentURL, err)
	}

	log.Debugf("Successfully clicked element '%s'.", elementSelector)
	return nil
}

func (m *Method) MouseClick(x, y float64) error {
	if time.Since(lastClickTime) < 400*time.Millisecond {
		log.Debugf("Click too fast, wait for 1s")
		time.Sleep(400 * time.Millisecond)
	}

	return chromedp.Run(m.pageCtx, chromedp.MouseClickXY(x, y))
}
