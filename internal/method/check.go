package method

import (
	"github.com/playwright-community/playwright-go"
)

func (m *Method) IsVisible(element playwright.Locator) (bool, error) {
	return element.IsVisible()
}

func (*Method) IsDisabled(element playwright.Locator) (bool, error) {
	return element.IsDisabled()
}
