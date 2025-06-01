package method

import (
	"time"
)

func (m *Method) SleepMilliseconds(milliseconds int) bool {
	time.Sleep(time.Duration(milliseconds) * time.Millisecond)
	return true
}
