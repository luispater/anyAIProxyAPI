package method

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) IsVisible(node *cdp.Node) (bool, error) {
	if node == nil {
		log.Debug("IsVisible cannot check visibility of a nil node")
		return false, fmt.Errorf("cannot check visibility of a nil node")
	}
	if node.NodeID == 0 { // Basic check for invalid node
		log.Debug("IsVisible node has an invalid NodeID (0) for IsVisible check")
		return false, fmt.Errorf("node has an invalid NodeID (0) for IsVisible check")
	}

	// Use chromedp.WaitVisible with a very short timeout to check for visibility.
	// If it returns an error, the element is likely not visible.
	ctx, cancel := context.WithTimeout(m.page.GetContext(), 1000*time.Millisecond) // Short timeout for a quick check
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(node.NodeID, chromedp.ByNodeID), // sel is node.NodeID, no By* option needed
	)

	if err != nil {
		// If the error is a timeout or context deadline, it means the element is not visible.
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return false, nil
		}
		// Any other error is unexpected.
		log.Debugf("IsVisible error for node ID %d: %v", node.NodeID, err)
		return false, fmt.Errorf("error during visibility check for node ID %d: %w", node.NodeID, err)
	}

	return true, nil // No error means it's visible
}

func (m *Method) IsVisibleBySelector(elementSelector string) (bool, error) {
	ctx, cancel := context.WithTimeout(m.page.GetContext(), 1000*time.Millisecond) // Short timeout for a quick check
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.WaitVisible(elementSelector, chromedp.ByQuery), // sel is node.NodeID, no By* option needed
	)

	if err != nil {
		// If the error is a timeout or context deadline, it means the element is not visible.
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return false, nil
		}
		// Any other error is unexpected.
		log.Debugf("IsVisible error for node ID %s: %v", elementSelector, err)
		return false, fmt.Errorf("error during visibility check for node ID %s: %w", elementSelector, err)
	}

	return true, nil // No error means it's visible
}

func (m *Method) IsDisabled(node *cdp.Node) (bool, error) {
	if node == nil {
		return false, fmt.Errorf("cannot check disabled state of a nil node")
	}
	if node.NodeID == 0 { // Basic check for invalid node
		return false, fmt.Errorf("node has an invalid NodeID (0) for IsDisabled check")
	}

	ok := true
	result := ""
	err := chromedp.Run(m.page.GetContext(),
		chromedp.AttributeValue([]cdp.NodeID{node.NodeID}, "disabled", &result, &ok, chromedp.ByNodeID), // ByNodeID is implicit when sel is NodeID
	)

	return ok, err
}
