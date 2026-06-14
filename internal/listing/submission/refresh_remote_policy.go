package submission

type RefreshRemotePolicy struct {
	DefaultConfirmed bool
	FallbackMessage  string
}

func BuildRefreshRemotePolicy(action string, publishAccepted bool) RefreshRemotePolicy {
	return RefreshRemotePolicy{
		DefaultConfirmed: action == "publish" && publishAccepted,
		// Preserve current refresh behavior; platform-specific remote resolution supplies
		// any publish fallback message when needed.
		FallbackMessage: "",
	}
}
