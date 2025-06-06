package method

import (
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) InInnerHTML(selector, htmlContent string) error {
	// This script will be executed in the browser's context.
	// It takes two arguments: the selector and the HTML content.
	jsScript := `function(selector, html) {
		const el = document.querySelector(selector);
		if (el) {
			el.innerHTML = html;
		}
	}`
	// We use CallFunctionOn to pass arguments to a JS function.
	// 'sel' is nil, so 'this' will be the global window object.
	// The result 'res' is also nil as we don't need a return value.
	err := chromedp.Run(m.pageCtx,
		chromedp.CallFunctionOn(jsScript, nil, nil, selector, htmlContent),
	)
	if err != nil {
		log.Error(err)
	}
	return err
}
