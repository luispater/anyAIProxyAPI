package method

import (
	"fmt"
	"github.com/chromedp/chromedp"
	"reflect"
	"strings"
	// playwright-go import will be removed if no longer needed by other files in this package
	// "github.com/playwright-community/playwright-go"
)

func (m *Method) AlwaysTrue() bool {
	return true
}

func (m *Method) GetLocalStorage(name string) (any, error) {
	var value any
	script := fmt.Sprintf(`localStorage.getItem('%s')`, name)
	err := chromedp.Run(m.page.GetContext(),
		chromedp.Evaluate(script, &value),
	)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (m *Method) SetLocalStorage(name, value string) error {
	script := fmt.Sprintf(`localStorage.setItem('%s', '%s')`, name, value)
	return chromedp.Run(m.page.GetContext(),
		chromedp.Evaluate(script, nil), // result is not needed for setItem
	)
}

func (m *Method) Int(i int) (int, error) {
	return i, nil
}

func (m *Method) Len(arr any) int {
	return reflect.ValueOf(arr).Len()
}

func (m *Method) ConvertReturnToParagraphs(input string) string {
	trimmedInput := strings.TrimSuffix(input, "\n")
	lines := strings.Split(trimmedInput, "\n")
	var result strings.Builder
	for _, line := range lines {
		result.WriteString("<p>")
		result.WriteString(line)
		result.WriteString("</p>")
	}
	return result.String()
}
