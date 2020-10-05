package server

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"kimo/types"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/log"
)

// TCPProxyRecord is type for defining a connection through TCP Proxy to MySQL
type TCPProxyRecord struct {
	ProxyInput   types.Addr
	ProxyOutput  types.Addr
	MysqlInput   types.Addr
	ClientOutput types.Addr
}
type TCPProxy struct {
	MgmtAddress string
	HttpClient  *http.Client
	Logger      log.Logger
}

func NewTCPProxy(mgmtAddress string, connectTimeout, readTimeout time.Duration) *TCPProxy {
	t := new(TCPProxy)
	t.MgmtAddress = mgmtAddress
	t.HttpClient = NewHttpClient(connectTimeout*time.Second, readTimeout*time.Second)
	return t
}

func (t *TCPProxy) FetchRecords(ctx context.Context, recordsC chan<- []*TCPProxyRecord, errC chan<- error) {
	url := fmt.Sprintf("http://%s/conns", t.MgmtAddress)
	t.Logger.Infof("Requesting to tcpproxy %s\n", url)
	response, err := t.HttpClient.Get(url)
	if err != nil {
		errC <- err
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Logger.Debugf("Error: %s\n", response.Status)
		errC <- errors.New("status code is not 200")
		return
	}

	// Read all the response body
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Logger.Errorln(err.Error())
		errC <- err
		return
	}

	parsedContents := strings.Split(string(contents), "\n")

	records := make([]*TCPProxyRecord, 0)
	for _, record := range parsedContents {
		addr, err := t.parseRecord(record)
		if err != nil {
			t.Logger.Debugf("record '%s' could not be parsed \n", record)
			continue
		}
		records = append(records, addr)
	}
	recordsC <- records
}

func (t *TCPProxy) parseRecord(record string) (*TCPProxyRecord, error) {
	// Sample Output:
	// 10.0.4.219:36149 -> 10.0.0.68:3306 -> 10.0.0.68:35423 -> 10.0.0.241:3306
	// <client>:<output_port> -> <proxy>:<input_port> -> <proxy>:<output_port>: -> <mysql>:<input_port>
	record = strings.TrimSpace(record)
	items := strings.Split(record, "->")
	var tcpAddr TCPProxyRecord
	for idx, item := range items {
		parts := strings.Split(strings.TrimSpace(item), ":")
		if len(parts) < 2 {
			return nil, errors.New("unknown")
		}

		p, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			t.Logger.Errorf("error during string to int32: %s\n", err)
			return nil, err
		}

		ip := parts[0]
		port := uint32(p)

		if idx == 0 {
			tcpAddr.ClientOutput.Host = ip
			tcpAddr.ClientOutput.Port = port
		} else if idx == 1 {
			tcpAddr.ProxyInput.Host = ip
			tcpAddr.ProxyInput.Port = port
		} else if idx == 2 {
			tcpAddr.ProxyOutput.Host = ip
			tcpAddr.ProxyOutput.Port = port
		} else if idx == 3 {
			tcpAddr.MysqlInput.Host = ip
			tcpAddr.MysqlInput.Port = port
		} else {
			return nil, errors.New("unknown")
		}
	}

	return &tcpAddr, nil
}
