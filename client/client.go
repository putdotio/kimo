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
	Config          *config.Client
	TcpProxy        *tcpproxy.TcpProxy
	TcpProxyRecords []types.TcpProxyRecord
}

func (c *Client) Run() error {
	// get mysql info
	mysqlProcesses, err := mysql.GetProcesses(c.Config.DSN)
	if err != nil {
		return err
	}

	// todo: should be run in parallel.
	proxyRecords, err := c.TcpProxy.GetRecords()
	if err != nil {
		// todo: handle error
	}
	c.TcpProxyRecords = proxyRecords
	fmt.Printf("Proxy addresses: %+v\n", c.TcpProxyRecords)

	kimoProcesses := make([]*types.KimoProcess, 0)
	// get server info
	for _, mp := range mysqlProcesses {
		var kp types.KimoProcess
		kp.MysqlProcess = mp
		// todo: debug log
		fmt.Printf("%+v\n", mp)
		// todo: use goroutine
		sp, err := c.GetServerProcess(&kp, mp.Host, mp.Port)
		if err != nil {
			// todo: store errors inside KimoProcess
			fmt.Println(err.Error())
			continue
		}
		fmt.Printf("Settin sp: %+v\n", sp)
		kp.ServerProcess = sp
		kimoProcesses = append(kimoProcesses, &kp)
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

func (c *Client) GetServerProcess(kp *types.KimoProcess, host string, port uint32) (*types.ServerProcess, error) {
	// todo: host validation
	// todo: server port as config or cli argument
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	// todo: http or https
	// todo: use port from config
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

	// todo: do not return list from server
	sp := ksr.ServerProcesses[0]
	sp.Hostname = ksr.Hostname

	if sp.Laddr.Port != port {
		return nil, errors.New("could not found")
	}

	if sp.Name != "tcpproxy" {
		return &sp, nil
	}

	kp.TcpProxyProcess = &sp
	pr, err := c.TcpProxy.GetProxyRecord(*kp.TcpProxyProcess, c.TcpProxyRecords)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	kp.TcpProxyRecord = pr
	return c.GetServerProcess(kp, pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)
}
