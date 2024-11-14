package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"kimo/types"
	"net"
	"net/http"

	"github.com/cenkalti/log"
)

// TCPProxyResult is type for defining a connection through TCP Proxy to MySQL
type TCPProxyConn struct {
	ClientOut types.IPPort `json:"client_out"`
	ProxyIn   types.IPPort `json:"proxy_in"`
	ProxyOut  types.IPPort `json:"proxy_out"`
	ServerIn  types.IPPort `json:"server_in"`
}

// TCPConnResponse is a type for TCP Proxy management api response
type TCPConnResponse struct {
	Records []*TCPProxyConn `json:"conns"`
}

// TCPProxyClient is used for getting info from tcp proxy
type TCPProxyClient struct {
	MgmtAddress string
}

// NewTCPProxy is used to create a new TCPProxy
func NewTCPProxyClient(cfg config.Server) *TCPProxyClient {
	tc := new(TCPProxyClient)
	tc.MgmtAddress = cfg.TCPProxyMgmtAddress
	return tc
}

// Get is used to fetch connection records from tcp proxy.
func (tc *TCPProxyClient) Get(ctx context.Context) ([]*TCPProxyConn, error) {
	url := fmt.Sprintf("http://%s/conns?json=true", tc.MgmtAddress)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		return nil, errors.New("status code is not 200")
	}

	var conns TCPConnResponse
	err = json.NewDecoder(response.Body).Decode(&conns)
	if err != nil {
		log.Errorln("Can not decode conns")
		return nil, errors.New("can not decode tcpproxy response")
	}
	return conns.Records, nil
}

func findHostIP(host string) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", err
		}
		return string(ips[0].String()), nil
	}
	return ip.String(), nil
}

func findTCPProxyConn(addr types.IPPort, proxyConns []*TCPProxyConn) *TCPProxyConn {
	ipAddr, err := findHostIP(addr.IP)
	if err != nil {
		log.Debugln(err.Error())
		return nil
	}

	for _, conn := range proxyConns {
		if conn.ProxyOut.IP == ipAddr && conn.ProxyOut.Port == addr.Port {
			return conn
		}
	}
	return nil
}
