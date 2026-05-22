package api

var newConcreteHandler func(HandlerService, ...HandlerOption) (*handler, error) = NewHandler
