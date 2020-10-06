package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type TCPConns struct {
	Records []*TCPProxyRecord `json:"conns"`
}

type TCPProxy struct {
	MgmtAddress string
	HttpClient  *http.Client
}

func NewTCPProxy(mgmtAddress string, connectTimeout, readTimeout time.Duration) *TCPProxy {
	t := new(TCPProxy)
	t.MgmtAddress = mgmtAddress
	t.HttpClient = NewHttpClient(connectTimeout*time.Second, readTimeout*time.Second)
	return t
}

func (t *TCPProxy) FetchRecords(ctx context.Context, recordsC chan<- []*TCPProxyRecord, errC chan<- error) {
	url := fmt.Sprintf("http://%s/conns?json=true", t.MgmtAddress)
	log.Infof("Requesting to tcpproxy %s\n", url)
	response, err := t.HttpClient.Get(url)
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
