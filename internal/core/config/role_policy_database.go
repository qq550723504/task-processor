package config

import "github.com/spf13/viper"

func rolePolicyDatabase(v *viper.Viper) *DatabaseConfig {
	if !v.IsSet("listingkit.rolePolicyDatabase") {
		return nil
	}
	var cfg DatabaseConfig
	if v.UnmarshalKey("listingkit.rolePolicyDatabase", &cfg) != nil {
		return &DatabaseConfig{}
	}
	return &cfg
}
