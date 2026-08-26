package httpapi

import (
	"sync"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
)

type zitadelAuthConfig = zitadelruntime.Config
type zitadelAuthorizationConfig = zitadelruntime.AuthorizationConfig

type listingKitZitadelRuntimeConfig struct {
	AuthConfig  zitadelAuthConfig
	AuthzConfig zitadelAuthorizationConfig
	Authorizer  *authz.ListingKitAuthorizer
}

var (
	listingKitZitadelRuntimeConfigMu sync.RWMutex
	listingKitZitadelRuntimeConfigV  *listingKitZitadelRuntimeConfig
)
