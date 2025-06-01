package method

import (
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/adapter"
	"github.com/luispater/anyAIProxyAPI/internal/browser/chrome"
)

func (m *Method) ResponseData(page *chrome.Page, channel chan *adapter.AdapterResponse) (bool, error) {
	data, err := page.ResponseData()
	if err != nil {
		return false, err
	}
	channel <- data
	if data.Done {
		return data.Done, nil
	} else {
		return false, fmt.Errorf("not finish yet")
	}
}
