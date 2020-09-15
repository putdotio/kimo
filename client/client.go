package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func NewClient(cfg *config.Client) *Client {
	client := new(Client)
	client.Config = cfg
	client.TcpProxy = tcpproxy.NewTcpProxy(cfg)
	return client
}

type Client struct {
	Config   *config.Client
	TcpProxy *tcpproxy.TcpProxy
}

func (c *Client) Run() error {
	// get mysql info
	mysqlProcesses, err := mysql.GetProcesses(c.Config.DSN)
	if err != nil {
		return err
	}

	kimoProcesses := make([]*types.KimoProcess, 0)
	// get server info
	for _, mp := range mysqlProcesses {
		var kp types.KimoProcess
		// todo: debug log
		fmt.Printf("%+v\n", mp)
		sp, err := c.GetServerProcesses(mp.Host, mp.Port)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		// todo: find a better way.
		if sp.Type == "tcpproxy" {
			kp.TcpProxyProcess = sp
		} else {
			kp.ServerProcess = sp
		}
		kp.MysqlProcess = mp
		kimoProcesses = append(kimoProcesses, &kp)
	}

	// todo: should be run in parallel.
	proxyAddresses, err := c.TcpProxy.GetAddresses()
	if err != nil {
		// todo: handle error
	}
	fmt.Printf("Proxy addresses: %+v\n", proxyAddresses)

	fmt.Printf("Getting real host ips for %d processes...\n", len(kimoProcesses))
	// todo: this should be recursive
	// set real host ip & address
	for _, kp := range kimoProcesses {
		fmt.Printf("KimoProcess: %+v\n", kp)
		if kp.TcpProxyProcess != nil {
			pr, nil := c.TcpProxy.GetProxyRecord(*kp.TcpProxyProcess, proxyAddresses)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			kp.TcpProxyRecord = pr
		}
	}

	// todo: DRY.
	// todo: we should find the real process recursive.
	for _, kp := range kimoProcesses {
		// todo: debug log
		sp, err := c.GetServerProcesses(kp.TcpProxyRecord.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)

		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		kp.ServerProcess = sp

	}
	for _, kp := range kimoProcesses {
		fmt.Printf("final kp: %+v\n", kp)
		fmt.Printf("final sp: %+v\n", kp.ServerProcess)
		fmt.Printf("final tp: %+v\n", kp.TcpProxyProcess)
		fmt.Printf("final mp: %+v\n", kp.MysqlProcess)
		fmt.Printf("final tcp: %+v\n", kp.TcpProxyRecord)
	}

	return nil
}

func (c *Client) GetServerProcesses(host string, port uint32) (*types.ServerProcess, error) {
	// todo: host validation
	// todo: server port as config or cli argument
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	// todo: http or https
	url := fmt.Sprintf("http://%s:3333/conns?ports=%d", host, port)
	// todo: use request with context
	// todo: timeout
	fmt.Println("Requesting to ", url)
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

	ksr := types.KimoServerResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	// todo: handle tcpproxy
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	for _, sp := range ksr.ServerProcesses {
		if sp.Laddr.Port == port {
			sp.Hostname = ksr.Hostname
			if sp.Name == "tcpproxy" {
				sp.Type = "tcpproxy"
			} else {
				sp.Type = "kimo-server"
			}
			return &sp, nil
		}
	}

	return nil, errors.New("could not found")
}
