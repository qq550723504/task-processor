package commercetool

var reservedAuthorityFields = [...]string{
	"tenant_id",
	"user_id",
	"roles",
	"permission",
	"call_id",
	"agent_id",
	"agent_version",
	"agent_run_id",
	"business_task_id",
	"trace_id",
	"idempotency_key",
	"tool_id",
	"tool_version",
}

func isReservedAuthorityField(name string) bool {
	for _, reserved := range reservedAuthorityFields {
		if name == reserved {
			return true
		}
	}

	return false
}
