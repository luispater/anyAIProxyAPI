package method

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
)

func (m *Method) Click(elementSelector string, timeout float64) error {
	log.Debugf("Attempting to find and click element with selector: %s", elementSelector)
	element := m.page.Locator(elementSelector).First() // Use First() to get the first match if multiple
	count, err := element.Count()
	if err != nil {
		return fmt.Errorf("error counting elements with selector '%s': %v", elementSelector, err)
	}
	if count > 0 {
		isVisible, errIsVisible := element.IsVisible()
		if errIsVisible != nil {
			return fmt.Errorf("error checking visibility of element '%s': %v", elementSelector, errIsVisible)
		} else if isVisible {
			log.Debugf("Element '%s' found and is visible. Attempting to click...", elementSelector)
			if err = element.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(timeout),
			}); err != nil {
				return fmt.Errorf("error clicking element '%s': %v", elementSelector, err)
			} else {
				log.Debugf("Successfully clicked element '%s'.", elementSelector)
			}
		} else {
			return fmt.Errorf("element '%s' found but is not visible", elementSelector)
		}
	} else {
		return fmt.Errorf("error: Element with selector '%s' not found on page %s", elementSelector, m.page.URL())
	}
	return nil
}

func (m *Method) MouseClick(x, y float64) error {
	return m.page.Mouse().Click(x, y)
}
