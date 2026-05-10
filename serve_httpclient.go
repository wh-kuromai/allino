package allino

import (
	"net/http"
	"time"
)

type HttpClientConfig struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

func (c *HttpClientConfig) setup() (*http.Client, error) {
	var transport http.Transport
	var valueSet bool

	if c.MaxIdleConns != 0 {
		transport.MaxIdleConns = c.MaxIdleConns
		valueSet = true
	}

	if c.MaxIdleConnsPerHost != 0 {
		transport.MaxIdleConnsPerHost = c.MaxIdleConnsPerHost
		valueSet = true
	}

	if c.IdleConnTimeout != 0 {
		transport.IdleConnTimeout = c.IdleConnTimeout
		valueSet = true
	}

	client := &http.Client{
		Timeout: c.Timeout,
	}

	if valueSet {
		client.Transport = &transport
	}

	return client, nil
}
