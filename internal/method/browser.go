package method

import (
	"github.com/chromedp/chromedp"
)

func (m *Method) GetURL() (string, error) {
	var currentURL string
	err := chromedp.Run(m.page.GetContext(), chromedp.Location(&currentURL))
	if err != nil {
		return "", err
	}
	return currentURL, nil
}
