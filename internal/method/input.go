package method

import (
	"context" // Added import
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) Input(elementSelector, text string, timeout float64) error {
	var nodes []*cdp.Node
	err := chromedp.Run(m.pageCtx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		chromedp.Run(m.pageCtx, chromedp.Location(&currentURL)) // Best effort to get URL
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	// Check visibility (optional, SendKeys might work on non-visible items or fail gracefully)
	// For simplicity, we'll rely on SendKeys' behavior. If strict visibility check is needed, add chromedp.Visible here.

	log.Debugf("Element '%s' found. Attempting to input...", elementSelector)
	actions := []chromedp.Action{
		// chromedp.Focus(elementSelector, chromedp.ByQuery), // Ensure focus before sending keys
		// chromedp.Clear(elementSelector, chromedp.ByQuery), // Clear field before input if necessary
		chromedp.SetValue(elementSelector, text, chromedp.ByQuery),
	}
	// Playwright's Fill usually overwrites, SendKeys appends.
	// If overwrite is needed, consider chromedp.SetValue or a sequence of Clear then SendKeys.
	// For now, SendKeys is the closest general equivalent.

	// Timeout in Playwright's Fill is for the action to complete.
	// Chromedp's Run command itself can be wrapped in a context with timeout.
	// If timeout here means a pause *after* action, then use Sleep.
	// For now, assuming timeout is for the operation itself, handled by m.pageCtx if it has a deadline.
	// If a specific timeout for *this* operation is needed, a new timed context should be derived from m.pageCtx.
	opCtx := m.pageCtx
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.pageCtx, time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error inputting into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully input into element '%s'.", elementSelector)
	return nil
}

func (m *Method) Type(elementSelector, text string, timeout float64) error {
	// In Chromedp, Type is often the same as Fill/SendKeys for text fields.
	// We'll use SendKeys which simulates typing.
	var nodes []*cdp.Node
	err := chromedp.Run(m.pageCtx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		chromedp.Run(m.pageCtx, chromedp.Location(&currentURL)) // Best effort
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	log.Debugf("Element '%s' found. Attempting to type...", elementSelector)
	actions := []chromedp.Action{
		// chromedp.Focus(elementSelector, chromedp.ByQuery), // Ensure focus
		chromedp.SendKeys(elementSelector, text, chromedp.ByQuery), // chromedp.KeyEvent to simulate individual key presses if needed
	}

	// Timeout handling: Playwright's Type has a timeout for the action.
	// Chromedp's SendKeys doesn't have a per-action timeout parameter.
	// The overall chromedp.Run(m.pageCtx, ...) will respect the deadline of m.pageCtx.
	// If a specific timeout for this operation is needed, derive a new context:
	opCtx := m.pageCtx
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.pageCtx, time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}
	// For now, we rely on m.pageCtx's timeout.

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error typing into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully typed into element '%s'.", elementSelector)
	return nil
}

func (m *Method) PressSequentially(elementSelector, text string, timeout float64) error {
	// PressSequentially implies typing character by character.
	// chromedp.SendKeys generally handles this well.
	var nodes []*cdp.Node
	err := chromedp.Run(m.pageCtx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("error finding element with selector '%s': %v", elementSelector, err)
	}
	if len(nodes) == 0 {
		var currentURL string
		chromedp.Run(m.pageCtx, chromedp.Location(&currentURL)) // Best effort
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}

	log.Debugf("Element '%s' found. Attempting to press sequentially...", elementSelector)
	// chromedp.SendKeys simulates this. For more granular control:
	// opActions := []chromedp.Action{chromedp.Focus(elementSelector, chromedp.ByQuery)}
	// for _, char := range text {
	// 	opActions = append(opActions, chromedp.KeyEvent(string(char)))
	//  // Potentially add small delays here if PressSequentially implies that
	// }
	// err = chromedp.Run(m.pageCtx, opActions...)

	actions := []chromedp.Action{
		// chromedp.Focus(elementSelector, chromedp.ByQuery),
		chromedp.SendKeys(elementSelector, text, chromedp.ByQuery),
	}
	// Timeout handling similar to Type method.
	opCtx := m.pageCtx
	var cancel context.CancelFunc
	if timeout > 0 {
		opCtx, cancel = context.WithTimeout(m.pageCtx, time.Duration(timeout*float64(time.Millisecond)))
		defer cancel()
	}

	if err = chromedp.Run(opCtx, actions...); err != nil {
		return fmt.Errorf("error pressing sequentially into element '%s': %v", elementSelector, err)
	}
	log.Debugf("Successfully pressed sequentially into element '%s'.", elementSelector)
	return nil
}
