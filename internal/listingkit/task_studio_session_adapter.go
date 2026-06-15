package listingkit

import (
	"github.com/google/uuid"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioSessionRunner = studiodomain.SessionService[
	SheinStudioSession,
	SheinStudioSelection,
	SheinStudioDesign,
]

type listingStudioSessionAsyncJobRunner = studiodomain.SessionAsyncJobSyncService[
	SheinStudioSession,
	SheinStudioSessionStatus,
]

type listingStudioSessionGenerationMetadataRunner = studiodomain.SessionGenerationMetadataService[
	SheinStudioSession,
	SheinStudioSessionStatus,
	SheinStudioGenerationJob,
]

type listingStudioSessionReviewTaskMetadataRunner = studiodomain.SessionReviewTaskMetadataService[
	SheinStudioSession,
	SheinStudioCreatedTask,
]

type listingStudioSessionGeneralMetadataRunner = studiodomain.SessionGeneralMetadataService[
	SheinStudioSession,
	UpdateStudioSessionRequest,
]

func newListingStudioSessionService(repo studioSessionDraftRepository) *listingStudioSessionRunner {
	return studiodomain.NewSessionService(studiodomain.SessionServiceConfig[
		SheinStudioSession,
		SheinStudioSelection,
		SheinStudioDesign,
	]{
		Repo:              studioSessionRepositoryAdapter{repo: repo},
		ValidateSelection: validateStudioSessionSelection,
		BuildSelectionKey: buildStudioSelectionKey,
		NewSession:        newListingStudioSessionRecord,
		SessionID: func(session *SheinStudioSession) string {
			if session == nil {
				return ""
			}
			return session.ID
		},
		RequestUserID: RequestUserIDFromContext,
		NewSessionID:  uuid.NewString,
	})
}

func newListingStudioSessionAsyncJobService(repo studioSessionDraftRepository) *listingStudioSessionAsyncJobRunner {
	return studiodomain.NewSessionAsyncJobSyncService(studiodomain.SessionAsyncJobSyncServiceConfig[
		SheinStudioSession,
		SheinStudioSessionStatus,
	]{
		Repo:               studioSessionMutationRepositoryAdapter{repo: repo},
		StatusForJob:       studioSessionStatusForAsyncJob,
		SetStatus:          setListingStudioSessionStatus,
		SetGenerationJob:   setListingStudioSessionGenerationJobID,
		SetGenerationError: setListingStudioSessionGenerationError,
	})
}

func newListingStudioSessionGenerationMetadataService(repo studioSessionDraftRepository) *listingStudioSessionGenerationMetadataRunner {
	return studiodomain.NewSessionGenerationMetadataService(studiodomain.SessionGenerationMetadataServiceConfig[
		SheinStudioSession,
		SheinStudioSessionStatus,
		SheinStudioGenerationJob,
	]{
		Repo:               studioSessionMutationRepositoryAdapter{repo: repo},
		SetStatus:          setListingStudioSessionStatus,
		SetGenerationJobID: setListingStudioSessionGenerationJobID,
		SetGenerationJobs:  setListingStudioSessionGenerationJobs,
		SetGenerationError: setListingStudioSessionGenerationError,
	})
}

func newListingStudioSessionReviewTaskMetadataService(repo studioSessionDraftRepository) *listingStudioSessionReviewTaskMetadataRunner {
	return studiodomain.NewSessionReviewTaskMetadataService(studiodomain.SessionReviewTaskMetadataServiceConfig[
		SheinStudioSession,
		SheinStudioCreatedTask,
	]{
		Repo:                 studioSessionMutationRepositoryAdapter{repo: repo},
		SetApprovedDesignIDs: setListingStudioSessionApprovedDesignIDs,
		SetCreatedTasks:      setListingStudioSessionCreatedTasks,
	})
}

func newListingStudioSessionGeneralMetadataService(repo studioSessionDraftRepository) *listingStudioSessionGeneralMetadataRunner {
	return studiodomain.NewSessionGeneralMetadataService(studiodomain.SessionGeneralMetadataServiceConfig[
		SheinStudioSession,
		UpdateStudioSessionRequest,
	]{
		Repo:       studioSessionMutationRepositoryAdapter{repo: repo},
		ApplyPatch: applyListingStudioSessionGeneralMetadataPatch,
	})
}
