package database

import "time"

// Config contains the runtime settings needed to open a PostgreSQL database.
type Config struct {
	Host                  string
	Port                  int
	User                  string
	Password              string
	Database              string
	MaxConnections        int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
}
