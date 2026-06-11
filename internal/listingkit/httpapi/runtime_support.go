package httpapi

import (
	"task-processor/internal/listingkit"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
)

type RuntimeSupportInput struct {
	SheinCookieStore          *sheinlogin.RedisStore
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
}

type RuntimeSupport struct {
	Repositories              BuildServiceRepositories
	Hooks                     BuildServiceHooks
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
}

func BuildRuntimeSupport(input RuntimeSupportInput) RuntimeSupport {
	return RuntimeSupport{
		Repositories:              buildRuntimeSupportRepositories(),
		Hooks:                     buildRuntimeSupportHooks(input.SheinCookieStore),
		SDSSyncService:            input.SDSSyncService,
		SDSLoginStatusProvider:    input.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: input.SDSBaselineRemoteProvider,
	}
}
