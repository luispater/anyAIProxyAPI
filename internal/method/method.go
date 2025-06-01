package method

import "github.com/playwright-community/playwright-go"

type Method struct {
	page playwright.Page
}

func NewMethod(page *playwright.Page) *Method {
	return &Method{
		page: *page,
	}
}
