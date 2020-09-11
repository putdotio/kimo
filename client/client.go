package client

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/tcpproxy"
	"kimo/types"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// query information_schema.processlist table
// collect data from servers
// display results

// todo: DRY.

func Run(host, user, password string) error {
	// get mysql info
	mysqlProcesses, err := getMysqlProcesses(host, user, password)
	if err != nil {
		return err
	}

	kimoProcesses := make([]*types.KimoProcess, 0)
	// get server info
	for _, proc := range mysqlProcesses {
		// todo: debug log
		fmt.Printf("%+v\n", proc)
		kp, err := getResponseFromServer(proc.Host, proc.Port)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		kimoProcesses = append(kimoProcesses, kp)
	}

	// todo: should be run in parallel.
	proxyAddresses, err := tcpproxy.GetResponseFromTcpProxy()
	if err != nil {
		// todo: handle error
	}
	fmt.Printf("Proxy addresses: %+v\n", proxyAddresses)

	fmt.Printf("Getting real host ips for %d processes...\n", len(kimoProcesses))
	// todo: this should be recursive
	// set real host ip & address
	for _, kimoProcess := range kimoProcesses {
		fmt.Printf("kimoProcess: %+v\n", kimoProcess)
		if kimoProcess.Name == "tcpproxy" {
			addr, nil := getRealHostAddr(*kimoProcess, proxyAddresses)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}

			// todo: can we handle this without overriding existing values?
			fmt.Printf("overriding %s -> %s && %d -> %d \n", kimoProcess.Laddr.IP, addr.IP, kimoProcess.Laddr.Port, addr.Port)
			kimoProcess.TcpProxies = append(kimoProcess.TcpProxies, types.Addr{kimoProcess.Laddr.IP, kimoProcess.Laddr.Port})
			kimoProcess.Laddr.IP = addr.IP
			kimoProcess.Laddr.Port = addr.Port
		}
	}

	for _, kp := range kimoProcesses {
		// todo: debug log
		fmt.Println("host2:", kp.Laddr.IP, "port2:", kp.Laddr.Port)
		// todo: bad naming
		res, err := getResponseFromServer(kp.Laddr.IP, kp.Laddr.Port)

		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		// override
		kp.Name = res.Name
		kp.Pid = res.Pid
		kp.Status = res.Status
		kp.Hostname = res.Hostname
		kp.CmdLine = res.CmdLine
	}
	for _, kp := range kimoProcesses {
		fmt.Printf("final: %+v\n", kp)
	}

	return nil
}

func getRealHostAddr(kimoProcess types.KimoProcess, proxyAddresses []tcpproxy.TcpProxyRecord) (*types.Addr, error) {
	fmt.Printf("looking for: %+v\n", kimoProcess)
	for _, proxyAddress := range proxyAddresses {
		fmt.Printf("proxyAddress: %+v\n", proxyAddress)
		if proxyAddress.ProxyOutput.IP == kimoProcess.Laddr.IP && proxyAddress.ProxyOutput.Port == kimoProcess.Laddr.Port {
			fmt.Println("found!")
			return &types.Addr{proxyAddress.ClientOutput.IP, proxyAddress.ClientOutput.Port}, nil
		}
	}
	fmt.Println("Could not found!")

	return nil, errors.New("could not found")

}

func portsAsString(ports []uint32) string {
	portsArray := make([]string, 0)
	for _, port := range ports {
		portsArray = append(portsArray, fmt.Sprint(port))
	}

	return strings.Join(portsArray, ",")

}

// todo: bad naming
func getResponseFromServer(host string, port uint32) (*types.KimoProcess, error) {
	// todo: host validation
	// todo: server port as config or cli argument
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	// todo: http or https
	url := fmt.Sprintf("http://%s:8090/conns?ports=%d", host, port)
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

	for _, kp := range ksr.KimoProcesses {
		if kp.Laddr.Port == port {
			kp.Hostname = ksr.Hostname
			return &kp, nil
		}
	}

	return nil, errors.New("could not found")
}

type mysqlProcess struct {
	ID      int32          `json:"id"`
	User    string         `json:"user"`
	Host    string         `json:"host"`
	Port    uint32         `json:"port"`
	DB      sql.NullString `json:"db"`
	Command string         `json:"command"`
	Time    string         `json:"time"`
	State   sql.NullString `json:"state"`
	Info    sql.NullString `json:"info"`
}

func getMysqlProcesses(host, user, password string) ([]mysqlProcess, error) {
	dsn := fmt.Sprintf("%s:%s@(%s:3306)/information_schema", user, password, host)
	db, err := sql.Open("mysql", dsn)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	results, err := db.Query("select * from PROCESSLIST")
	if err != nil {
		return nil, err
	}

	mysqlProcesses := make([]mysqlProcess, 0)
	for results.Next() {
		var mysqlProcess mysqlProcess
		var host string

		err = results.Scan(&mysqlProcess.ID, &mysqlProcess.User, &host, &mysqlProcess.DB, &mysqlProcess.Command,
			&mysqlProcess.Time, &mysqlProcess.State, &mysqlProcess.Info)

		if err != nil {
			return nil, err
		}
		fmt.Println("host: ", host)
		s := strings.Split(host, ":")
		if len(s) < 2 {
			// it might be localhost
			continue
		}
		parsedPort, err := strconv.ParseInt(s[1], 10, 32)
		if err != nil {
			log.Printf("error during string to int32: %s\n", err)
			continue
		}
		mysqlProcess.Host = s[0]
		mysqlProcess.Port = uint32(parsedPort)
		mysqlProcesses = append(mysqlProcesses, mysqlProcess)
	}

	return mysqlProcesses, nil
}
