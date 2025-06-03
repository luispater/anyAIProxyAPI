package method

import log "github.com/sirupsen/logrus"

func (m *Method) InInnerHTML(selector, htmlContent string) error {
	jsScript := `
		(args) => {
			const el = document.querySelector(args[0]);
			if (el) {
				el.innerHTML = args[1];
			}
			return el;
		}
	`
	_, err := m.page.Evaluate(jsScript, []any{selector, htmlContent})
	if err != nil {
		log.Error(err)
	}
	return err
}
