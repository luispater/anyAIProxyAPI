package method

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
)

func (m *Method) GetElements(elementSelector string) ([]playwright.Locator, error) {
	elementsLocator := m.page.Locator(elementSelector)
	count, err := elementsLocator.Count()
	if err != nil {
		log.Debugf("Error counting elements with selector '%s': %v", elementSelector, err)
	}
	if count > 0 {
		return elementsLocator.All()
	} else {
		return nil, fmt.Errorf("error: Elements with selector '%s' not found on page %s", elementSelector, m.page.URL())
	}
}

func (m *Method) GetElement(elementSelector string) (playwright.Locator, error) {
	elementsLocator := m.page.Locator(elementSelector)
	count, err := elementsLocator.Count()
	if err != nil {
		log.Debugf("Error counting elements with selector '%s': %v", elementSelector, err)
	}
	if count > 0 {
		return elementsLocator.First(), nil
	} else {
		return nil, fmt.Errorf("error: Elements with selector '%s' not found on page %s", elementSelector, m.page.URL())
	}
}

func (m *Method) GetElementAttribute(locator playwright.Locator, attributeName string) (string, error) {
	return locator.GetAttribute(attributeName)
}

func (m *Method) GetInnerText(elementSelector string) (string, error) {
	element, err := m.GetElement(elementSelector)
	if err != nil {
		return "", nil
	}
	innerText, err := element.InnerText()
	if err != nil {
		log.Debugf("GetInnerText error: %v", err)
	} else {
		log.Debugf("GetInnerText: %s", innerText)
	}

	return innerText, err
}
