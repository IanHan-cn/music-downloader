package music

import (
	"net"
	"net/http"
	"time"
)

var sharedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          1000,
	MaxIdleConnsPerHost:   100,
	MaxConnsPerHost:       100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
}

var (
	downloadClient = &http.Client{Transport: sharedTransport, Timeout: 5 * time.Minute}
	apiClient      = &http.Client{Transport: sharedTransport, Timeout: 15 * time.Second}
)
