package adapter

import (
	"github.com/luispater/anyAIProxyAPI/internal/proxy/model"
	"github.com/luispater/anyAIProxyAPI/internal/proxy/utils"
)

var Adapters = map[string]Adapter{}

type Adapter interface {
	ShouldRecord(buffer []byte) bool
	HandleResponse(responseBuffer chan []byte, disconnect chan bool, sniffing *bool, queue *utils.Queue[*model.ProxyResponse])
}
