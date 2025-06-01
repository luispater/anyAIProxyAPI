package method

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
)

func (m *Method) Value(elementSelector string, timeout float64) (string, error) {
	element := m.page.Locator(elementSelector).First() // Use First() to get the first match if multiple
	count, err := element.Count()
	if err != nil {
		return "", fmt.Errorf("error counting elements with selector '%s': %v", elementSelector, err)
	}
	if count > 0 {
		return element.InputValue(playwright.LocatorInputValueOptions{
			Timeout: playwright.Float(timeout),
		})
	} else {
		return "", fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, m.page.URL())
	}
}
