package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	Raddr  gopsutilNet.Addr `json:"remoteaddr"`
	Status string           `json:"status"`
	Pid    int32            `json:"pid"`
	// CmdLine string  `json:"cmdline"`  // how to get this?
	Name string `json:"name"`
}

type KimoServerResponse struct {
	Hostname      string        `json:"hostname"`
	KimoProcesses []KimoProcess `json:"processes"`
}

func Run(host, user, password string) error {
	mysqlProcesses, err := getMysqlProcesses(host, user, password)
	if err != nil {
		return err
	}
	// get mysql processes
	// group by host-port
	// send requests to hosts

	for _, proc := range mysqlProcesses {
		// todo: debug log
		fmt.Printf("%+v\n", proc)
	}

	m := groupByHost(mysqlProcesses)

	serverResponses := make([]KimoServerResponse, 1)
	fmt.Println("grouped:")
	for host, ports := range m {
		// todo: debug log
		fmt.Println("host:", host, "ports:", ports)
		// todo: naming
		ksr, err := getResponseFromServer(host, ports)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		serverResponses = append(serverResponses, *ksr)
	}

	fmt.Println("Server Responses:")
	for _, sr := range serverResponses {
		fmt.Printf("%+v\n", sr)
	}

	return nil
}

func portsAsString(ports []uint32) string {
	portsArray := make([]string, 1)
	for _, port := range ports {
		portsArray = append(portsArray, fmt.Sprint(port))
	}

	return strings.Join(portsArray, ",")

}

func getResponseFromServer(host string, ports []uint32) (*KimoServerResponse, error) {
	// todo: host validation
	// todo: server port as config or cli argument
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	// todo: http or
	url := fmt.Sprintf("https://%s:8090/conns?ports=", host, portsAsString(ports))
	// todo: use request with context
	// todo: timeout
	fmt.Println("Requesting to ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	ksr := KimoServerResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	// todo: handle tcpproxy

	return &ksr, nil
}

func groupByHost(mysqlProcesses []mysqlProcess) map[string][]uint32 {
	m := make(map[string][]uint32)

	for _, proc := range mysqlProcesses {
		if val, ok := m[proc.Host]; ok {
			m[proc.Host] = append(val, proc.Port)
		} else {
			m[proc.Host] = make([]uint32, 0)
			m[proc.Host] = append(m[proc.Host], proc.Port)
		}
	}

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

	mysqlProcesses := make([]mysqlProcess, 1)
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
