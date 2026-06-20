package scheduler

type ManagementRuntime = managementRuntime

func NewManagementRuntime(runtime managementRuntime) ManagementRuntime {
	if runtime == nil {
		return nil
	}
	return runtime
}
