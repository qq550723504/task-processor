package authorizedbrand

import (
	"strings"

	"task-processor/internal/listingruntime"
)

type Config struct {
	Enabled bool
	Code    string
	Name    string
}

type Resolved struct {
	Enabled bool
	Code    string
	Name    string
	NameEn  string
}

func ConfigFromStore(store *listingruntime.StoreInfo) Config {
	if store == nil || store.EnableBrandAuthorization == nil || !*store.EnableBrandAuthorization {
		return Config{}
	}

	return Config{
		Enabled: true,
		Code:    strings.TrimSpace(store.AuthorizedBrandCode),
		Name:    strings.TrimSpace(store.AuthorizedBrandName),
	}
}
