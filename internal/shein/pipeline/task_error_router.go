package pipeline

import (
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api"
)

type taskErrorRoute int

const (
	taskErrorRouteFailure taskErrorRoute = iota
	taskErrorRouteAuthenticationExpired
)

type taskErrorDecision struct {
	route   taskErrorRoute
	err     error
	authErr *api.AuthenticationExpiredError
}

type TaskErrorRouter struct{}

func NewTaskErrorRouter() *TaskErrorRouter {
	return &TaskErrorRouter{}
}

func (r *TaskErrorRouter) Route(task model.Task, err error) taskErrorDecision {
	if cookieErr, isCookieError := shein.IsCookieLoadError(err); isCookieError {
		return taskErrorDecision{
			route: taskErrorRouteFailure,
			err:   cookieErr,
		}
	}

	if authErr, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
		return taskErrorDecision{
			route:   taskErrorRouteAuthenticationExpired,
			authErr: authErr,
		}
	}

	if shein.IsAuthenticationExpiredError(err) {
		return taskErrorDecision{
			route: taskErrorRouteAuthenticationExpired,
			authErr: &api.AuthenticationExpiredError{
				TenantID: task.TenantID,
				ShopID:   task.StoreID,
				Code:     "20302",
				Message:  err.Error(),
			},
		}
	}

	return taskErrorDecision{
		route: taskErrorRouteFailure,
		err:   err,
	}
}
