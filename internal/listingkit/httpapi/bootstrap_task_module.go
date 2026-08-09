package httpapi

import (
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/tenantdirectory"
	"task-processor/internal/listingkit/zitadelsms"
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
	ZitadelSMSService               *zitadelsms.Service
}

type taskModule struct {
	taskRepository      listingkit.Repository
	handlerDependencies listingkitapi.HandlerDependencies
}

func newTaskModuleInput(input BuildServiceInput, repos *builtRepositories) taskModuleInput {
	var directory tenantdirectory.Directory
	var invitationProvider memberinvite.Provider
	var zitadelSMSService *zitadelsms.Service
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
		zitadelSMSService = buildZitadelSMSService(zitadelConfig.SMS)
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
		ZitadelSMSService:               zitadelSMSService,
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
			ZitadelSMSService: in.ZitadelSMSService,
		},
	}
}

func buildZitadelSMSService(cfg config.ListingKitZitadelSMSConfig) *zitadelsms.Service {
	sender, err := zitadelsms.NewTencentSender(cfg.TencentSecretID, cfg.TencentSecretKey)
	if err != nil {
		return nil
	}
	service, err := zitadelsms.NewService(zitadelsms.Config{
		SigningKey: cfg.SigningKey,
		TemplateID: cfg.TencentTemplateID,
		SignName:   cfg.TencentSignName,
		AppID:      cfg.TencentAppID,
	}, sender)
	if err != nil {
		return nil
	}
	return service
}

func (m taskModule) handlerDependenciesWithAdmin(admin adminModule) listingkitapi.HandlerDependencies {
	deps := m.handlerDependencies
	deps.Admin = admin.handlerDependencies
	return deps
}
