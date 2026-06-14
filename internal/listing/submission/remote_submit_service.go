package submission

import "context"

type RemoteSubmitInput[TPackage, TProductAPI, TProduct, TSnapshot any] struct {
	TaskID     string
	Package    TPackage
	Action     string
	RequestID  string
	ProductAPI TProductAPI
	Product    TProduct
	Snapshot   TSnapshot
}

type RemoteSubmitResult[TResponse, TSnapshot any] struct {
	SupplierCode string
	Response     TResponse
	Snapshot     TSnapshot
	Err          error
}

type RemoteSubmitService[TPackage, TProductAPI, TProduct, TResponse, TSnapshot any] struct {
	prepareState   func(TPackage, string, string, TProduct, TSnapshot) (string, TSnapshot)
	executeAttempt func(context.Context, RemoteSubmitInput[TPackage, TProductAPI, TProduct, TSnapshot]) RemoteSubmitResult[TResponse, TSnapshot]
}

type RemoteSubmitServiceConfig[TPackage, TProductAPI, TProduct, TResponse, TSnapshot any] struct {
	PrepareState   func(TPackage, string, string, TProduct, TSnapshot) (string, TSnapshot)
	ExecuteAttempt func(context.Context, RemoteSubmitInput[TPackage, TProductAPI, TProduct, TSnapshot]) RemoteSubmitResult[TResponse, TSnapshot]
}

func NewRemoteSubmitService[TPackage, TProductAPI, TProduct, TResponse, TSnapshot any](config RemoteSubmitServiceConfig[TPackage, TProductAPI, TProduct, TResponse, TSnapshot]) *RemoteSubmitService[TPackage, TProductAPI, TProduct, TResponse, TSnapshot] {
	return &RemoteSubmitService[TPackage, TProductAPI, TProduct, TResponse, TSnapshot]{
		prepareState:   config.PrepareState,
		executeAttempt: config.ExecuteAttempt,
	}
}

func (s *RemoteSubmitService[TPackage, TProductAPI, TProduct, TResponse, TSnapshot]) Submit(
	ctx context.Context,
	in RemoteSubmitInput[TPackage, TProductAPI, TProduct, TSnapshot],
) RemoteSubmitResult[TResponse, TSnapshot] {
	var result RemoteSubmitResult[TResponse, TSnapshot]
	if s == nil {
		return result
	}
	if s.prepareState != nil {
		result.SupplierCode, result.Snapshot = s.prepareState(in.Package, in.Action, in.RequestID, in.Product, in.Snapshot)
	}
	if s.executeAttempt != nil {
		attempt := s.executeAttempt(ctx, in)
		result.Response = attempt.Response
		result.Err = attempt.Err
		if any(attempt.Snapshot) != nil {
			result.Snapshot = attempt.Snapshot
		}
	}
	return result
}
