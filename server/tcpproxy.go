package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"kimo/types"
	"net/http"
	"time"

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
	HTTPClient  *http.Client
}

// NewTCPProxy is used to create a new TCPProxy
func NewTCPProxyClient(cfg config.Server) *TCPProxyClient {
	tc := new(TCPProxyClient)
	tc.MgmtAddress = cfg.TCPProxyMgmtAddress
	tc.HTTPClient = NewHTTPClient(cfg.TCPProxyConnectTimeout*time.Second, cfg.TCPProxyReadTimeout*time.Second)
	return tc
}

// Get is used to fetch connection records from tcp proxy.
func (tc *TCPProxyClient) Get(ctx context.Context, recordsC chan<- []*TCPProxyConn, errC chan<- error) {
	url := fmt.Sprintf("http://%s/conns?json=true", tc.MgmtAddress)
	log.Infof("Requesting to tcpproxy %s\n", url)
	response, err := tc.HTTPClient.Get(url)
	if err != nil {
		errC <- err
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		errC <- errors.New("status code is not 200")
		return
	}

	var conns TCPConnResponse
	err = json.NewDecoder(response.Body).Decode(&conns)
	if err != nil {
		log.Errorln("Can not decode conns")
		errC <- errors.New("can not decode tcpproxy response")
	}
	log.Infof("Got %d TCP proxy records \n", len(conns.Records))
	recordsC <- conns.Records
}
