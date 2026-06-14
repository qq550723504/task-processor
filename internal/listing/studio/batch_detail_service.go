package studio

import "context"

type BatchDetailService[Graph any, Detail any] struct {
	loadGraph           func(context.Context, string) (*Graph, error)
	isGraphMissing      func(error) bool
	resolveWithoutGraph func(context.Context, string) (*Detail, bool, error)
	ensureGraph         func(context.Context, string) error
	projectDetail       func(context.Context, string, *Graph) (*Detail, error)
}

type BatchDetailServiceConfig[Graph any, Detail any] struct {
	LoadGraph           func(context.Context, string) (*Graph, error)
	IsGraphMissing      func(error) bool
	ResolveWithoutGraph func(context.Context, string) (*Detail, bool, error)
	EnsureGraph         func(context.Context, string) error
	ProjectDetail       func(context.Context, string, *Graph) (*Detail, error)
}

func NewBatchDetailService[Graph any, Detail any](config BatchDetailServiceConfig[Graph, Detail]) *BatchDetailService[Graph, Detail] {
	return &BatchDetailService[Graph, Detail]{
		loadGraph:           config.LoadGraph,
		isGraphMissing:      config.IsGraphMissing,
		resolveWithoutGraph: config.ResolveWithoutGraph,
		ensureGraph:         config.EnsureGraph,
		projectDetail:       config.ProjectDetail,
	}
}

func (s *BatchDetailService[Graph, Detail]) GetDetail(ctx context.Context, batchID string) (*Detail, error) {
	graph, err := s.loadGraph(ctx, batchID)
	if err != nil && s.isGraphMissing != nil && s.isGraphMissing(err) {
		fallbackDetail, syncRequired, syncErr := s.resolveWithoutGraph(ctx, batchID)
		if syncErr != nil {
			return nil, syncErr
		}
		if !syncRequired {
			return fallbackDetail, nil
		}
		if err := s.ensureGraph(ctx, batchID); err != nil {
			return nil, err
		}
		graph, err = s.loadGraph(ctx, batchID)
	}
	if err != nil {
		return nil, err
	}
	return s.projectDetail(ctx, batchID, graph)
}
