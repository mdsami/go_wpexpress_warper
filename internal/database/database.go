// Package database opens and verifies the MySQL connection pool.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/mdsami/wpexpress-warper/internal/config"
)

// New builds the pool and verifies the connection, mirroring the startup
// check in src/config/db.js.
func New(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	dsnCfg := mysql.NewConfig()
	dsnCfg.User = cfg.DBUser
	dsnCfg.Passwd = cfg.DBPassword
	dsnCfg.Net = "tcp"
	dsnCfg.Addr = net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort))
	dsnCfg.DBName = cfg.DBName
	dsnCfg.ParseTime = true // DATETIME columns scan into time.Time

	loc, err := location(cfg.DBTimezone)
	if err != nil {
		return nil, err
	}
	dsnCfg.Loc = loc

	db, err := sql.Open("mysql", dsnCfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// Equivalent of mysql2's connectionLimit: 10 / waitForConnections: true.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// location resolves DB_TIMEZONE, the zone DATETIME columns are read in.
// "UTC" (the default) treats stored values as UTC, which is how WordPress
// writes them. "Local" reproduces the Node version's behaviour, where
// mysql2 interprets them in the server's local zone.
func location(name string) (*time.Location, error) {
	switch name {
	case "", "UTC":
		return time.UTC, nil
	case "Local":
		return time.Local, nil
	default:
		loc, err := time.LoadLocation(name)
		if err != nil {
			return nil, fmt.Errorf("invalid DB_TIMEZONE %q: %w", name, err)
		}
		return loc, nil
	}
}
