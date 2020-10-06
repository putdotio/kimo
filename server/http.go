package server

import (
	"net"
	"net/http"
	"time"
)

// NewHTTPClient returns a http client with custom connect & read timeout
func NewHTTPClient(connectTimeout, readTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: TimeoutDialer(connectTimeout, readTimeout),
		},
	}
}

// TimeoutDialer is used to set connect & read timeouts for the client
func TimeoutDialer(connectTimeout, readTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, connectTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(readTimeout))
		return conn, nil
	}
}
