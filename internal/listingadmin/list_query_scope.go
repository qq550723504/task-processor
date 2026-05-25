package listingadmin

type listScopeAssignable interface {
	applyListScope(scope listQueryScope)
}

func applyListQueryScope[T listScopeAssignable](query T, scope listQueryScope) T {
	query.applyListScope(scope)
	return query
}

func (q *CategoryQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
}

func (q *FilterRuleQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *ImportTaskQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *OperationStrategyQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *PricingRuleQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *ProductDataQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *ProductImportMappingQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *ProfitRuleQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *SensitiveWordQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *StoreQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
	q.Page = scope.Page
	q.PageSize = scope.PageSize
}

func (q *StoreStatisticsQuery) applyListScope(scope listQueryScope) {
	q.TenantID = scope.TenantID
	q.OwnerUserID = scope.OwnerUserID
}
