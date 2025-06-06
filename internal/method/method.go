package method

import (
	"context"
	// "github.com/playwright-community/playwright-go" // Will be removed later if no longer needed by other functions
)

type Method struct {
	pageCtx context.Context
	// manager *chromedpmanager.Manager // Decided against including manager for now
}

func NewMethod(pCtx context.Context) *Method {
	return &Method{
		pageCtx: pCtx,
	}
}
