package method

import (
	"context"
	"fmt"
	"time"

	// Corrected import path for cdp types
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) GetElements(elementSelector string) ([]*cdp.Node, error) {
	var nodes []*cdp.Node
	ctx, cancel := context.WithTimeout(m.page.GetContext(), 1*time.Second)
	err := chromedp.Run(ctx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQueryAll),
	)
	defer cancel()
	if err != nil {
		// This error typically occurs if the selector is invalid or the page is not accessible.
		return nil, fmt.Errorf("error querying nodes with selector '%s': %v", elementSelector, err)
	}

	if len(nodes) == 0 {
		var currentURL string
		// Best effort to get URL for context in error message
		_ = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		log.Debugf("No elements found with selector '%s' on page %s", elementSelector, currentURL)
		// To maintain consistency with Playwright's behavior where getting a non-existent locator
		// doesn't error until an action is performed, we might return an empty slice and nil error.
		// However, the original code did error if count was 0.
		// Let's stick to erroring if not found, as it's generally safer.
		return nil, fmt.Errorf("error: Elements with selector '%s' not found on page %s", elementSelector, currentURL)
	}
	return nodes, nil
}

func (m *Method) GetElement(elementSelector string) (*cdp.Node, error) {
	var nodes []*cdp.Node
	// ByQuery by default gets the first element.
	// Using AtLeast(0) so it doesn't error if not found by Nodes action itself, we check len(nodes) ourselves.
	ctx, cancel := context.WithTimeout(m.page.GetContext(), 3*time.Second)
	err := chromedp.Run(ctx,
		chromedp.Nodes(elementSelector, &nodes, chromedp.ByQueryAll),
	)
	defer cancel()
	if err != nil {
		return nil, fmt.Errorf("error querying node with selector '%s': %v", elementSelector, err)
	}

	if len(nodes) == 0 {
		var currentURL string
		_ = chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
		log.Debugf("No element found with selector '%s' on page %s", elementSelector, currentURL)
		return nil, fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, currentURL)
	}
	return nodes[0], nil
}

// GetElementAttribute retrieves an attribute from a specific node.
// The signature has changed: it now takes a *cdp.Node.
func (m *Method) GetElementAttribute(node *cdp.Node, attributeName string) (string, error) {
	if node == nil {
		return "", fmt.Errorf("cannot get attribute from a nil node")
	}
	if node.NodeID == 0 {
		// This can happen if the node is detached or not properly initialized by Chromedp.
		// Attempt to re-fetch based on selector if available, though selector isn't passed here.
		// For now, error out. A more robust solution might involve node.FullSelector() if available.
		return "", fmt.Errorf("node has an invalid NodeID (0)")
	}
	log.Debugf("GetElementAttribute for node ID %d and attribute '%s'", node.NodeID, attributeName)
	var value string
	var found bool
	err := chromedp.Run(m.page.GetContext(),
		// Using node.NodeID to get the attribute.
		// The first argument `sel` can be a cdp.NodeID directly.
		// The `nil` is for the optional `ok *bool` parameter.
		// No need for ByNodeID option here as the type of sel (cdp.NodeID) implies it.
		chromedp.AttributeValue(cdp.NodeID(node.NodeID), attributeName, &value, &found),
	)
	log.Debugf("GetElementAttribute result: %s, found: %t, err: %v", value, found, err)
	return value, err
}

func (m *Method) GetElementAttributeBySelector(elementSelector string, attributeName string) (string, error) {
	var value string
	var found bool
	err := chromedp.Run(m.page.GetContext(),
		// Using node.NodeID to get the attribute.
		// The first argument `sel` can be a cdp.NodeID directly.
		// The `nil` is for the optional `ok *bool` parameter.
		// No need for ByNodeID option here as the type of sel (cdp.NodeID) implies it.
		chromedp.AttributeValue(elementSelector, attributeName, &value, &found, chromedp.ByQuery),
	)
	log.Debugf("GetElementAttribute result: %s, found: %t, err: %v", value, found, err)
	return value, err
}

func (m *Method) GetInnerText(elementSelector string) (string, error) {
	var innerText string
	// chromedp.Text gets the combined text content of the element and its descendants.
	err := chromedp.Run(m.page.GetContext(), chromedp.Text(elementSelector, &innerText, chromedp.ByQuery))
	if err != nil {
		log.Debugf("GetInnerText error for selector '%s': %v", elementSelector, err)
		return "", err // Propagate the error
	}
	log.Debugf("GetInnerText for selector '%s': %s", elementSelector, innerText)
	return innerText, nil // err is nil if chromedp.Run was successful
}
