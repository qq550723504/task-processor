package local

import (
	"task-processor/internal/core/config"
	"task-processor/internal/product"
)

type DataProvider = LocalDataProvider
type Runtime = LocalRuntime
type RuntimeOptions = LocalRuntimeOptions

func NewDataProvider(dbCfg *config.DatabaseConfig, redisCfg *config.RedisConfig) (*DataProvider, error) {
	return NewLocalDataProvider(dbCfg, redisCfg)
}

func NewRuntime(provider *DataProvider, options RuntimeOptions) *Runtime {
	return NewLocalRuntime(provider, options)
}

func NewRawJSONDataAdapter(provider *DataProvider) product.RawJsonDataClient {
	return NewRawJsonDataAdapter(provider)
}
