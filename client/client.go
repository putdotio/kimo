package client

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	gopsutilNet "github.com/shirou/gopsutil/net"
)

// query information_schema.processlist table
// collect data from servers
// display results

// todo: DRY.
type KimoProcess struct {
	Laddr  gopsutilNet.Addr `json:"localaddr"`
	Status string           `json:"status"`
	Pid    int32            `json:"pid"`
	// CmdLine string  `json:"cmdline"`  // how to get this?
	Name       string `json:"name"`
	TcpProxies []Addr
	Hostname   string
}

type KimoServerResponse struct {
	Hostname      string        `json:"hostname"`
	KimoProcesses []KimoProcess `json:"processes"`
}

func Run(host, user, password string) error {
	// get mysql info
	mysqlProcesses, err := getMysqlProcesses(host, user, password)
	if err != nil {
		return err
	}

	kimoProcesses := make([]*KimoProcess, 0)
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
	proxyAddresses, err := getResponseFromTcpProxy()
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
			kimoProcess.TcpProxies = append(kimoProcess.TcpProxies, Addr{kimoProcess.Laddr.IP, kimoProcess.Laddr.Port})
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
	}
	for _, kp := range kimoProcesses {
		fmt.Printf("final: %+v\n", kp)
	}

	return nil
}

func getRealHostAddr(kimoProcess KimoProcess, proxyAddresses []TcpProxyRecord) (*Addr, error) {
	fmt.Printf("looking for: %+v\n", kimoProcess)
	for _, proxyAddress := range proxyAddresses {
		fmt.Printf("proxyAddress: %+v\n", proxyAddress)
		if proxyAddress.proxyOutput.IP == kimoProcess.Laddr.IP && proxyAddress.proxyOutput.Port == kimoProcess.Laddr.Port {
			fmt.Println("found!")
			return &Addr{proxyAddress.clientOutput.IP, proxyAddress.clientOutput.Port}, nil
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
func getResponseFromServer(host string, port uint32) (*KimoProcess, error) {
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

	ksr := KimoServerResponse{}
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

type Addr struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}

type TcpProxyRecord struct {
	proxyInput   Addr
	proxyOutput  Addr
	mysqlInput   Addr
	clientOutput Addr
}

func getResponseFromTcpProxy() ([]TcpProxyRecord, error) {
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
			tcpAddr.clientOutput.IP = parts[0]
			tcpAddr.clientOutput.Port = uint32(port)
		} else if idx == 1 {
			tcpAddr.proxyInput.IP = parts[0]
			tcpAddr.proxyInput.Port = uint32(port)
		} else if idx == 2 {
			tcpAddr.proxyOutput.IP = parts[0]
			tcpAddr.proxyOutput.Port = uint32(port)
		} else if idx == 3 {
			tcpAddr.mysqlInput.IP = parts[0]
			tcpAddr.mysqlInput.Port = uint32(port)
		}
	}

	return &tcpAddr, nil
}

// todo: instead of a concrete type, there should be an interface like Process" and we should accept that as param
//       so, we can use this function for both of mysqlProcess and KimoProcess types.
func groupByHost(mysqlProcesses []mysqlProcess) map[string][]uint32 {
	m := make(map[string][]uint32)

	for _, proc := range mysqlProcesses {
		val, ok := m[proc.Host]
		fmt.Println("proc.Host:", proc.Host)
		if ok {
			m[proc.Host] = append(val, proc.Port)
			fmt.Println("xx:", proc.Port)
		} else {
			m[proc.Host] = []uint32{proc.Port}
			fmt.Println("yy:", proc.Port)
		}
	}

	fmt.Printf("%+v\n", m)
	return m

}

// todo: DRY.
func groupByHost2(kimoProcesses []*KimoProcess) map[string][]uint32 {
	m := make(map[string][]uint32)

	for _, proc := range kimoProcesses {
		val, ok := m[proc.Laddr.IP]
		fmt.Println("proc.Host:", proc.Laddr.IP)
		if ok {
			m[proc.Laddr.IP] = append(val, proc.Laddr.Port)
			fmt.Println("xx:", proc.Laddr.Port)
		} else {
			m[proc.Laddr.IP] = []uint32{proc.Laddr.Port}
			fmt.Println("yy:", proc.Laddr.Port)
		}
	}

	fmt.Printf("%+v\n", m)
	return m

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
