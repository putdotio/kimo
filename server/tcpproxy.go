package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"net"
	"net/http"

	"github.com/cenkalti/log"
)

// TCPProxyResult represents connection through TCPProxy to MySQL
type TCPProxyConn struct {
	ClientOut IPPort `json:"client_out"`
	ProxyIn   IPPort `json:"proxy_in"`
	ProxyOut  IPPort `json:"proxy_out"`
	ServerIn  IPPort `json:"server_in"`
}

// TCPConnResponse represents TCPProxy management API response
type TCPConnResponse struct {
	Records []*TCPProxyConn `json:"conns"`
}

// TCPProxyClient represents a TCPProxy client that manages connection details and stores TCPProxy management results.
type TCPProxyClient struct {
	MgmtAddress string
}

// NewServer creates an returns a new *TCPProxyClient
func NewTCPProxyClient(cfg config.TCPProxy) *TCPProxyClient {
	tc := new(TCPProxyClient)
	tc.MgmtAddress = cfg.MgmtAddress
	return tc
}

// Get gets connection records from TCPProxy.
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

func findTCPProxyConn(addr IPPort, proxyConns []*TCPProxyConn) *TCPProxyConn {
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
