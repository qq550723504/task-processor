package httpapi

import (
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/tenantdirectory"
	"task-processor/internal/listingsubscription"
)

type taskModuleInput struct {
	TaskRepository           listingkit.Repository
	StudioAsyncJobRepository listingkit.StudioAsyncJobRepository
	SubscriptionService      *listingsubscription.Service
	PlatformAdminUsers       []string
	PlatformAdminRoles       []string
	TenantDirectory          tenantdirectory.Directory
}

type taskModule struct {
	taskRepository      listingkit.Repository
	handlerDependencies listingkitapi.HandlerDependencies
}

func newTaskModuleInput(input BuildServiceInput, repos *builtRepositories) taskModuleInput {
	var directory tenantdirectory.Directory
	if input.Config != nil {
		directory, _ = tenantdirectory.NewClient(tenantdirectory.ClientConfig{
			IssuerURL: input.Config.ListingKit.Zitadel.IssuerURL,
			Token:     input.Config.ListingKit.Zitadel.TenantDirectoryToken,
		})
	}
	return taskModuleInput{
		TaskRepository:           repos.taskRepository,
		StudioAsyncJobRepository: repos.studioAsyncJobRepository,
		SubscriptionService:      repos.subscriptionService,
		PlatformAdminUsers:       append([]string{}, input.Config.ListingKit.PlatformAdminUsers...),
		PlatformAdminRoles:       append([]string{}, input.Config.ListingKit.PlatformAdminRoles...),
		TenantDirectory:          directory,
	}
}

func buildTaskModule(in taskModuleInput) taskModule {
	return taskModule{
		taskRepository: in.TaskRepository,
		handlerDependencies: listingkitapi.HandlerDependencies{
			StudioAsyncJobRepository: in.StudioAsyncJobRepository,
			Subscription: listingkitapi.SubscriptionDependencies{
				Service:            in.SubscriptionService,
				PlatformAdminUsers: append([]string{}, in.PlatformAdminUsers...),
				PlatformAdminRoles: append([]string{}, in.PlatformAdminRoles...),
				TenantDirectory:    in.TenantDirectory,
			},
		},
	}
}

func (m taskModule) handlerDependenciesWithAdmin(admin adminModule) listingkitapi.HandlerDependencies {
	deps := m.handlerDependencies
	deps.Admin = admin.handlerDependencies
	return deps
}
