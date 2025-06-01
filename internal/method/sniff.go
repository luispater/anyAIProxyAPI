package method

import (
	"fmt"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/proxy"
)

func (m *Method) StartSniffing(proxy *proxy.Proxy) {
	proxy.StartSniffing()
}

func (m *Method) StopSniffing(proxy *proxy.Proxy) {
	proxy.StopSniffing()
}

func (m *Method) GetDataFromProxy(proxy *proxy.Proxy, channel chan *model.ProxyResponse) (bool, error) {
	data, err := proxy.GetData()
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
