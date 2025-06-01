package method

import (
	"context" // Added import
	"fmt"
	"github.com/chromedp/chromedp/kb"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) Input(elementSelector, text string, timeout float64) error {
	var nodes []*cdp.Node
	err := chromedp.Run(m.page.GetContext(),
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		err = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL)) // Best effort to get URL
		if err == nil {
			return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
		}
		return fmt.Errorf("error: Element with selector '%s' not found on page", elementSelector)
	}

	log.Debugf("Element '%s' found. Attempting to input...", elementSelector)
	actions := []chromedp.Action{
		chromedp.SetValue(elementSelector, text, chromedp.ByQuery),
		chromedp.SendKeys(elementSelector, "1", chromedp.ByQuery),
		chromedp.SendKeys(elementSelector, kb.Backspace, chromedp.ByQuery),
	}

	opCtx := m.page.GetContext()
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.page.GetContext(), time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error inputting into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully input into element '%s'.", elementSelector)
	return nil
}

func (m *Method) Type(elementSelector, text string, timeout float64) error {
	var nodes []*cdp.Node
	err := chromedp.Run(m.page.GetContext(),
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		err = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		if err != nil {
			return fmt.Errorf("error: Element with selector '%s' not found on page", elementSelector)
		}
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	log.Debugf("Element '%s' found. Attempting to type...", elementSelector)
	actions := []chromedp.Action{
		chromedp.SendKeys(elementSelector, text, chromedp.ByQuery),
	}

	opCtx := m.page.GetContext()
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.page.GetContext(), time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error typing into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully typed into element '%s'.", elementSelector)
	return nil
}

func (m *Method) PressSequentially(elementSelector, text string, timeout float64) error {
	var nodes []*cdp.Node
	err := chromedp.Run(m.page.GetContext(),
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		err = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		if err != nil {
			return fmt.Errorf("error: Element with selector '%s' not found on page", elementSelector)
		}
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	log.Debugf("Element '%s' found. Attempting to press sequentially...", elementSelector)

	actions := []chromedp.Action{
		chromedp.SendKeys(elementSelector, text, chromedp.ByQuery),
	}

	opCtx := m.page.GetContext()
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.page.GetContext(), time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error pressing sequentially into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully pressed sequentially into element '%s'.", elementSelector)
	return nil
}
