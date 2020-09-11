package tcpproxy

import (
	"errors"
	"fmt"
	"io/ioutil"
	"kimo/types"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TcpProxyRecord struct {
	ProxyInput   types.Addr
	ProxyOutput  types.Addr
	MysqlInput   types.Addr
	ClientOutput types.Addr
}

func GetResponseFromTcpProxy() ([]TcpProxyRecord, error) {
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	// todo: tcpproxy url as config
	url := fmt.Sprintf("http://tcpproxy:3307/conns")
	fmt.Println("Requesting to tcpproxy ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		return nil, errors.New("status code is not 200")
	}

	// Read all the response body
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	records := strings.Split(string(contents), "\n")

	addresses := make([]TcpProxyRecord, 0)
	for _, record := range records {
		fmt.Println("record: ", record)
		addr, err := parseTcpProxyRecord(record)
		if err != nil {
			// todo: debug log
			continue
		}
		if addr == nil {
			// todo: debug log
			continue
		}
		addresses = append(addresses, *addr)
	}
	return addresses, nil
}

func parseTcpProxyRecord(record string) (*TcpProxyRecord, error) {
	// Sample Output:
	// 10.0.4.219:36149 -> 10.0.0.68:3306 -> 10.0.0.68:35423 -> 10.0.0.241:3306
	// <client>:<output_port> -> <proxy>:<input_port> -> <proxy>:<output_port>: -> <mysql>:<input_port>
	record = strings.TrimSpace(record)
	items := strings.Split(record, "->")
	var tcpAddr TcpProxyRecord
	for idx, item := range items {
		hostURL := strings.TrimSpace(item)
		parts := strings.Split(hostURL, ":")
		// todo: we should not need this. handle.
		if len(parts) < 2 {
			return nil, nil
		}
		port, err := strconv.ParseInt(parts[1], 10, 32)

		if err != nil {
			fmt.Printf("error during string to int32: %s\n", err)
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
		}
	}

	return &tcpAddr, nil
}
