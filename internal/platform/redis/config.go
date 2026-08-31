package redis

// Config defines the platform Redis connection settings.
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}
