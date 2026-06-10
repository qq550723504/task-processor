package listingkit

import (
	"errors"

	"task-processor/internal/listingkit/submission"
)

const sheinSubmitInFlightTTL = submission.InFlightTTL

var (
	errSheinSubmitReplayExisting = errors.New("shein submit replay existing")
	errSheinSubmitRecoverRemote  = errors.New("shein submit recover remote")
	errSheinSubmitMissingPackage = errors.New("shein submit missing package")
)
