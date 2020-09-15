package mysql

import (
	"database/sql"
	"kimo/types"
	"log"
	"strconv"
	"strings"
)

func NewMysql(dsn string) *Mysql {
	m := new(Mysql)
	m.DSN = dsn
	return m
}

type Mysql struct {
	DSN string
}

func (m *Mysql) GetProcesses() ([]types.MysqlProcess, error) {
	db, err := sql.Open("mysql", m.DSN)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	results, err := db.Query("select * from PROCESSLIST")
	if err != nil {
		return nil, err
	}
	mps := make([]types.MysqlProcess, 0)
	for results.Next() {
		var mp types.MysqlProcess
		var host string

		err = results.Scan(&mp.ID, &mp.User, &host, &mp.DB, &mp.Command, &mp.Time, &mp.State, &mp.Info)

		if err != nil {
			return nil, err
		}
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
		mp.Host = s[0]
		mp.Port = uint32(parsedPort)
		mps = append(mps, mp)
	}
	return mps, nil
}
