package server

import (
	"context"
	"database/sql"
	"kimo/types"
	"strconv"
	"strings"

	"github.com/cenkalti/log"
	_ "github.com/go-sql-driver/mysql" // imports mysql driver
)

// MysqlProcess is the process type in terms of MySQL context (a row from processlist table)
type MysqlProcess struct {
	ID      int32          `json:"id"`
	User    string         `json:"user"`
	DB      sql.NullString `json:"db"`
	Command string         `json:"command"`
	Time    string         `json:"time"`
	State   sql.NullString `json:"state"`
	Info    sql.NullString `json:"info"`
	Address types.IPPort   `json:"address"`
}

// NewMysql is used to create a Mysql type.
func NewMysql(dsn string) *Mysql {
	m := new(Mysql)
	m.DSN = dsn
	return m
}

// Mysql is used to get processes from mysql.
type Mysql struct {
	DSN       string
	Processes []MysqlProcess
}

// FetchProcesses is used to fetch processlist table from information_schema.
func (m *Mysql) FetchProcesses(ctx context.Context, procsC chan<- []*MysqlProcess, errC chan<- error) {
	log.Infoln("Requesting to mysql...")
	db, err := sql.Open("mysql", m.DSN)

	if err != nil {
		errC <- err
	}
	defer db.Close()

	results, err := db.Query("select * from PROCESSLIST")
	if err != nil {
		errC <- err
	}
	mps := make([]*MysqlProcess, 0)
	for results.Next() {
		var mp MysqlProcess
		var host string

		err = results.Scan(&mp.ID, &mp.User, &host, &mp.DB, &mp.Command, &mp.Time, &mp.State, &mp.Info)

		if err != nil {
			errC <- err
		}
		s := strings.Split(host, ":")
		if len(s) < 2 {
			// it might be localhost
			continue
		}
		parsedPort, err := strconv.ParseInt(s[1], 10, 32)
		if err != nil {
			log.Errorf("error during string to int32: %s\n", err)
			continue
		}
		mp.Address = types.IPPort{
			IP:   s[0],
			Port: uint32(parsedPort),
		}
		mps = append(mps, &mp)
	}
	log.Infof("Got %d mysql processes \n", len(mps))
	procsC <- mps
}
