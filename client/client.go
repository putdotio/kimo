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
	client.Mysql = mysql.NewMysql(cfg.DSN)
	client.TcpProxy = tcpproxy.NewTcpProxy(cfg)
	client.KimoProcessChan = make(chan types.KimoProcess)
	return client
}

type Client struct {
	Config          *config.Client
	Mysql           *mysql.Mysql
	TcpProxy        *tcpproxy.TcpProxy
	TcpProxyRecords []types.TcpProxyRecord
	KimoProcessChan chan types.KimoProcess
}

func (c *Client) Run() error {
	// todo: use context
	mysqlProcesses, err := c.Mysql.GetProcesses()
	if err != nil {
		return err
	}
	// todo: use context
	proxyRecords, err := c.TcpProxy.GetRecords()
	if err != nil {
		// todo: handle error
	}
	c.TcpProxyRecords = proxyRecords
	// get server info
	for _, mp := range mysqlProcesses {
		fmt.Printf("mp: %+v\n", mp)
		var kp types.KimoProcess
		kp.MysqlProcess = mp
		// todo: debug log
		// todo: use wait group
		go c.GetServerProcess(&kp, mp.Host, mp.Port)
	}
	<-c.KimoProcessChan

	for kp := range c.KimoProcessChan {
		fmt.Printf("final kp: %+v\n", kp)
		fmt.Printf("final sp: %+v\n", kp.ServerProcess)
		fmt.Printf("final tp: %+v\n", kp.TcpProxyProcess)
		fmt.Printf("final mp: %+v\n", kp.MysqlProcess)
		fmt.Printf("final tcp: %+v\n", kp.TcpProxyRecord)
	}

	return nil
}

func (c *Client) GetServerProcess(kp *types.KimoProcess, host string, port uint32) error {
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
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		return errors.New("status code is not 200")
	}

	ksr := types.KimoServerResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	// todo: handle tcpproxy
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	// todo: do not return list from server
	sp := ksr.ServerProcesses[0]
	sp.Hostname = ksr.Hostname

	if sp.Laddr.Port != port {
		return errors.New("could not found")
	}

	if sp.Name != "tcpproxy" {
		kp.ServerProcess = &sp
		c.KimoProcessChan <- *kp
		return nil
	}

	kp.TcpProxyProcess = &sp
	pr, err := c.TcpProxy.GetProxyRecord(*kp.TcpProxyProcess, c.TcpProxyRecords)
	if err != nil {
		fmt.Println(err.Error())
		c.KimoProcessChan <- *kp
		return err
	}
	kp.TcpProxyRecord = pr
	err = c.GetServerProcess(kp, pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)
	if err != nil {
		fmt.Println(err.Error())
		c.KimoProcessChan <- *kp
		return err
	}
	return nil
}
