package config

import "time"

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host                  string        `mapstructure:"host" yaml:"host"`
	Port                  int           `mapstructure:"port" yaml:"port"`
	User                  string        `mapstructure:"user" yaml:"user"`
	Password              string        `mapstructure:"password" yaml:"password"`
	Database              string        `mapstructure:"database" yaml:"database"`
	MaxConnections        int           `mapstructure:"max_connections" yaml:"max_connections"`
	MaxIdleConnections    int           `mapstructure:"max_idle_connections" yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connection_max_lifetime" yaml:"connection_max_lifetime"`
}
