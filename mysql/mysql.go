package mysql

import (
	"database/sql"
	"fmt"
	"kimo/types"
	"log"
	"strconv"
	"strings"
)

func GetProcesses(dsn string) ([]*types.MysqlProcess, error) {
	db, err := sql.Open("mysql", dsn)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	results, err := db.Query("select * from PROCESSLIST")
	if err != nil {
		return nil, err
	}

	mysqlProcesses := make([]*types.MysqlProcess, 0)
	for results.Next() {
		var mysqlProcess types.MysqlProcess
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
		mysqlProcesses = append(mysqlProcesses, &mysqlProcess)
	}

	return mysqlProcesses, nil
}
