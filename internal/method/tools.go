package method

import (
	"fmt"
	"reflect"
	"strings"
)

func (m *Method) AlwaysTrue() bool {
	return true
}

func (m *Method) GetLocalStorage(name string) (any, error) {
	value, err := m.page.Evaluate(fmt.Sprintf(`() => localStorage.getItem('%s')`, name))
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (m *Method) SetLocalStorage(name, value string) error {
	_, err := m.page.Evaluate(fmt.Sprintf(`() => {localStorage.setItem('%s', '%s')}`, name, value))
	return err
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
