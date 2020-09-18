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
	Records     []types.TcpProxyRecord
}

func NewTcpProxy(mgmtAddress string) *TcpProxy {
	t := new(TcpProxy)
	t.MgmtAddress = mgmtAddress
	t.HttpClient = &http.Client{Timeout: 2 * time.Second}
	return t
}

func (t *TcpProxy) Setup(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/conns", t.MgmtAddress)
	log.Debugf("Requesting to tcpproxy %s\n", url)
	response, err := t.HttpClient.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		return errors.New("status code is not 200")
	}

	// Read all the response body
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}

	parsedContents := strings.Split(string(contents), "\n")

	t.Records = make([]types.TcpProxyRecord, 0)
	for _, record := range parsedContents {
		addr, err := t.parseRecord(record)
		if err != nil {
			log.Debugf("record could not be parsed %s \n", record)
			continue
		}
		t.Records = append(t.Records, *addr)
	}
	log.Infof("got all (%d) records\n", len(t.Records))
	return nil
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
			tcpAddr.ClientOutput.IP = ip
			tcpAddr.ClientOutput.Port = port
		} else if idx == 1 {
			tcpAddr.ProxyInput.IP = ip
			tcpAddr.ProxyInput.Port = port
		} else if idx == 2 {
			tcpAddr.ProxyOutput.IP = ip
			tcpAddr.ProxyOutput.Port = port
		} else if idx == 3 {
			tcpAddr.MysqlInput.IP = ip
			tcpAddr.MysqlInput.Port = port
		} else {
			return nil, errors.New("unknown")
		}
	}

	return &tcpAddr, nil
}

func (t *TcpProxy) GetProxyRecord(dp types.DaemonProcess, proxyRecords []types.TcpProxyRecord) (*types.TcpProxyRecord, error) {
	for _, pr := range proxyRecords {
		if pr.ProxyOutput.IP == dp.Laddr.IP && pr.ProxyOutput.Port == dp.Laddr.Port {
			return &pr, nil
		}
	}
	return nil, errors.New("could not found")
}
