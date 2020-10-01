package tcpproxy

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kimo/types"

	"github.com/cenkalti/log"
)

type TcpProxy struct {
	MgmtAddress string
	HttpClient  *http.Client
}

func NewTcpProxy(mgmtAddress string) *TcpProxy {
	t := new(TcpProxy)
	t.MgmtAddress = mgmtAddress
	t.HttpClient = &http.Client{Timeout: 2 * time.Second}
	return t
}

func (t *TcpProxy) FetchRecords(ctx context.Context, recordsC chan<- []*types.TcpProxyRecord, errC chan<- error) {
	url := fmt.Sprintf("http://%s/conns", t.MgmtAddress)
	log.Debugf("Requesting to tcpproxy %s\n", url)
	response, err := t.HttpClient.Get(url)
	if err != nil {
		errC <- err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		errC <- errors.New("status code is not 200")
	}

	// Read all the response body
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(err.Error())
		errC <- err
	}

	parsedContents := strings.Split(string(contents), "\n")

	records := make([]*types.TcpProxyRecord, 0)
	for _, record := range parsedContents {
		addr, err := t.parseRecord(record)
		if err != nil {
			log.Debugf("record could not be parsed %s \n", record)
			continue
		}
		records = append(records, addr)
	}
	recordsC <- records
}

func (t *TcpProxy) parseRecord(record string) (*types.TcpProxyRecord, error) {
	// Sample Output:
	// 10.0.4.219:36149 -> 10.0.0.68:3306 -> 10.0.0.68:35423 -> 10.0.0.241:3306
	// <client>:<output_port> -> <proxy>:<input_port> -> <proxy>:<output_port>: -> <mysql>:<input_port>
	record = strings.TrimSpace(record)
	items := strings.Split(record, "->")
	var tcpAddr types.TcpProxyRecord
	for idx, item := range items {
		parts := strings.Split(strings.TrimSpace(item), ":")
		if len(parts) < 2 {
			return nil, errors.New("unknown")
		}

		p, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			log.Errorf("error during string to int32: %s\n", err)
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
