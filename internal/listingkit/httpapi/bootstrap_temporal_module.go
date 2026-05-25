package httpapi

type temporalModuleInput struct {
	Service moduleService
}

type temporalModule struct {
	workerService TemporalWorkerService
}

func buildTemporalModule(in temporalModuleInput) temporalModule {
	return temporalModule{
		workerService: in.Service,
	}
}
