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

// TCPProxyRecord is type for defining a connection through TCP Proxy to MySQL
type TCPProxyRecord struct {
	ClientOut types.IPPort `json:"client_out"`
	ProxyIn   types.IPPort `json:"proxy_in"`
	ProxyOut  types.IPPort `json:"proxy_out"`
	ServerIn  types.IPPort `json:"server_in"`
}

// TCPConns is a type for TCP Proxy management api response
type TCPConns struct {
	Records []*TCPProxyRecord `json:"conns"`
}

// TCPProxy is used for getting info from tcp proxy
type TCPProxy struct {
	MgmtAddress string
	HTTPClient  *http.Client
}

// NewTCPProxy is used to create a new TCPProxy
func NewTCPProxy(cfg config.Server) *TCPProxy {
	t := new(TCPProxy)
	t.MgmtAddress = cfg.TCPProxyMgmtAddress
	t.HTTPClient = NewHTTPClient(cfg.TCPProxyConnectTimeout*time.Second, cfg.TCPProxyReadTimeout*time.Second)
	return t
}

// Fetch is used to fetch connection records from tcp proxy.
func (t *TCPProxy) Fetch(ctx context.Context, recordsC chan<- []*TCPProxyRecord, errC chan<- error) {
	url := fmt.Sprintf("http://%s/conns?json=true", t.MgmtAddress)
	log.Infof("Requesting to tcpproxy %s\n", url)
	response, err := t.HTTPClient.Get(url)
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

	var conns TCPConns
	err = json.NewDecoder(response.Body).Decode(&conns)
	log.Infof("Got %d TCP proxy records \n", len(conns.Records))
	recordsC <- conns.Records
}
