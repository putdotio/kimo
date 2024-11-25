package server

import (
	"context"
	"database/sql"
	"kimo/config"
	"strconv"
	"strings"

	"github.com/cenkalti/log"
	_ "github.com/go-sql-driver/mysql" // imports mysql driver
)

// MysqlRow represents a row from processlist table
type MysqlRow struct {
	ID      int32          `json:"id"`
	User    string         `json:"user"`
	DB      sql.NullString `json:"db"`
	Command string         `json:"command"`
	Time    string         `json:"time"`
	State   sql.NullString `json:"state"`
	Info    sql.NullString `json:"info"`
	Address IPPort         `json:"address"`
}

// NewMysqlClient creates and returns a new *MysqlClient.
func NewMysqlClient(cfg config.MySQLConfig) *MysqlClient {
	m := new(MysqlClient)
	m.DSN = cfg.DSN
	return m
}

// MysqlClient represents a MySQL database client that manages connection details and stores query results.
type MysqlClient struct {
	DSN       string
	MysqlRows []MysqlRow
}

// Get gets  processlist table from information_schema.
func (mc *MysqlClient) Get(ctx context.Context) ([]*MysqlRow, error) {
	db, err := sql.Open("mysql", mc.DSN)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	results, err := db.QueryContext(ctx, "select * from PROCESSLIST")
	if err != nil {
		return nil, err
	}

	if results == nil {
		return nil, err
	}

	mps := make([]*MysqlRow, 0)
	for results.Next() {
		var mp MysqlRow
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
			log.Errorf("error during string to int32: %s\n", err)
			continue
		}
		mp.Address = IPPort{
			IP:   s[0],
			Port: uint32(parsedPort),
		}
		mps = append(mps, &mp)
	}
	return mps, nil
}
