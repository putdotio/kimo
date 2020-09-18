package tcpproxy

import (
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

func (t *TcpProxy) GetRecords() error {
	url := fmt.Sprintf("http://%s/conns", t.MgmtAddress)
	log.Debugf("Requesting to tcpproxy %s\n", url)
	response, err := t.HttpClient.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		// todo: return appropriate error
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
			// todo: debug log
			continue
		}
		t.Records = append(t.Records, *addr)
	}
	log.Infoln("got all records")
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
		hostURL := strings.TrimSpace(item)
		parts := strings.Split(hostURL, ":")
		// todo: we should not need this. handle.
		if len(parts) < 2 {
			// todo: define proper error
			return nil, errors.New("unknown")
		}
		port, err := strconv.ParseInt(parts[1], 10, 32)

		if err != nil {
			log.Errorf("error during string to int32: %s\n", err)
			// todo: handle error and return zero value of Addr
		}
		// todo: DRY.
		if idx == 0 {
			tcpAddr.ClientOutput.IP = parts[0]
			tcpAddr.ClientOutput.Port = uint32(port)
		} else if idx == 1 {
			tcpAddr.ProxyInput.IP = parts[0]
			tcpAddr.ProxyInput.Port = uint32(port)
		} else if idx == 2 {
			tcpAddr.ProxyOutput.IP = parts[0]
			tcpAddr.ProxyOutput.Port = uint32(port)
		} else if idx == 3 {
			tcpAddr.MysqlInput.IP = parts[0]
			tcpAddr.MysqlInput.Port = uint32(port)
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
