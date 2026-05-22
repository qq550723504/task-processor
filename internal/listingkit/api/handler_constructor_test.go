package api

var newConcreteHandler func(routeHandlerService, ...HandlerOption) (*handler, error) = NewHandler
