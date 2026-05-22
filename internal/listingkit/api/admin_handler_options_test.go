package api

var _ HandlerOption = withStoreAdminDependencies(AdminHandlerDependencies{})
var _ HandlerOption = withCatalogAdminDependencies(AdminHandlerDependencies{})
