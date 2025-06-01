package method

import (
	"encoding/json"
	"fmt"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
)

func (m *Method) InInnerHTML(selector, htmlContent string) error {
	// This script will be executed in the browser's context.
	// It takes two arguments: the selector and the HTML content.
	jsScript := `var el = document.querySelector(%s);
		if (el) {
			el.innerHTML = %s;
		}`
	byteSelector, _ := json.Marshal(selector)
	bytehtmlContent, _ := json.Marshal(htmlContent)
	jsScript = fmt.Sprintf(jsScript, string(byteSelector), string(bytehtmlContent))
	// We use CallFunctionOn to pass arguments to a JS function.
	// 'sel' is nil, so 'this' will be the global window object.
	// The result 'res' is also nil as we don't need a return value.
	err := chromedp.Run(m.page.GetContext(),
		chromedp.Evaluate(jsScript, nil),
	)
	if err != nil {
		log.Error(err)
	}
	return err
}
