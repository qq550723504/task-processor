package httpapi

import (
	"strings"

	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/tenantdirectory"
	"task-processor/internal/listingsubscription"
)

type taskModuleInput struct {
	TaskRepository                  listingkit.Repository
	StudioAsyncJobRepository        listingkit.StudioAsyncJobRepository
	SubscriptionService             *listingsubscription.Service
	PlatformAdminUsers              []string
	PlatformAdminRoles              []string
	TenantDirectory                 tenantdirectory.Directory
	MemberInvitationProvider        memberinvite.Provider
	MemberInvitationAuditRepository memberinvite.AuditRepository
}

type taskModule struct {
	taskRepository      listingkit.Repository
	handlerDependencies listingkitapi.HandlerDependencies
}

func newTaskModuleInput(input BuildServiceInput, repos *builtRepositories) taskModuleInput {
	var directory tenantdirectory.Directory
	var invitationProvider memberinvite.Provider
	if input.Config != nil {
		directory, _ = tenantdirectory.NewClient(tenantdirectory.ClientConfig{
			IssuerURL: input.Config.ListingKit.Zitadel.IssuerURL,
			Token:     input.Config.ListingKit.Zitadel.TenantDirectoryToken,
		})
		zitadelConfig := input.Config.ListingKit.Zitadel
		if strings.TrimSpace(zitadelConfig.IssuerURL) != "" && strings.TrimSpace(zitadelConfig.MemberInvitationToken) != "" && strings.TrimSpace(zitadelConfig.ProjectID) != "" {
			invitationProvider, _ = memberinvite.NewZitadelProvider(memberinvite.ZitadelConfig{
				IssuerURL: zitadelConfig.IssuerURL,
				Token:     zitadelConfig.MemberInvitationToken,
				ProjectID: zitadelConfig.ProjectID,
			})
		}
	}
	var audit memberinvite.AuditRepository
	if repos != nil {
		audit = repos.memberInvitationAuditRepository
	}
	var platformAdminUsers, platformAdminRoles []string
	if input.Config != nil {
		platformAdminUsers = append([]string{}, input.Config.ListingKit.PlatformAdminUsers...)
		platformAdminRoles = append([]string{}, input.Config.ListingKit.PlatformAdminRoles...)
	}
	return taskModuleInput{
		TaskRepository:                  repos.taskRepository,
		StudioAsyncJobRepository:        repos.studioAsyncJobRepository,
		SubscriptionService:             repos.subscriptionService,
		PlatformAdminUsers:              platformAdminUsers,
		PlatformAdminRoles:              platformAdminRoles,
		TenantDirectory:                 directory,
		MemberInvitationProvider:        invitationProvider,
		MemberInvitationAuditRepository: audit,
	}
}

func buildTaskModule(in taskModuleInput) taskModule {
	return taskModule{
		taskRepository: in.TaskRepository,
		handlerDependencies: listingkitapi.HandlerDependencies{
			StudioAsyncJobRepository: in.StudioAsyncJobRepository,
			Subscription: listingkitapi.SubscriptionDependencies{
				Service:                         in.SubscriptionService,
				PlatformAdminUsers:              append([]string{}, in.PlatformAdminUsers...),
				PlatformAdminRoles:              append([]string{}, in.PlatformAdminRoles...),
				TenantDirectory:                 in.TenantDirectory,
				MemberInvitationProvider:        in.MemberInvitationProvider,
				MemberInvitationAuditRepository: in.MemberInvitationAuditRepository,
			},
		},
	}
}

func (m taskModule) handlerDependenciesWithAdmin(admin adminModule) listingkitapi.HandlerDependencies {
	deps := m.handlerDependencies
	deps.Admin = admin.handlerDependencies
	return deps
}
