package allino

import (
	"sync"

	"github.com/valyala/fasthttp"
)

var hostClientsMap sync.Map // map[string]*fasthttp.HostClient

func getHostClient(host string) *fasthttp.HostClient {
	if v, ok := hostClientsMap.Load(host); ok {
		return v.(*fasthttp.HostClient)
	}

	client := &fasthttp.HostClient{
		Addr:               host,
		StreamResponseBody: true,
	}

	actual, _ := hostClientsMap.LoadOrStore(host, client)
	return actual.(*fasthttp.HostClient)
}
