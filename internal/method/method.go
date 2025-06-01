package method

import (
	"github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
	// "github.com/playwright-community/playwright-go" // Will be removed later if no longer needed by other functions
)

type Method struct {
	page *chrome.Page
	// manager *chromedpmanager.Manager // Decided against including manager for now
}

func NewMethod(page *chrome.Page) *Method {
	return &Method{
		page: page,
	}
}
