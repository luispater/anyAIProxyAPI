package method

func (m *Method) Gt(a, b int) bool {
	return a > b
}

func (m *Method) Gte(a, b int) bool {
	return a >= b
}

func (m *Method) Lt(a, b int) bool {
	return a < b
}

func (m *Method) Lte(a, b int) bool {
	return a < b
}

func (m *Method) Eq(a, b int) bool {
	return a == b
}
