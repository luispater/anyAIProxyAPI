package method

import (
	"fmt"
	"reflect"
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
