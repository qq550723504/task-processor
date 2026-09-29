package httpapi

import (
	"errors"
	"task-processor/internal/httproute"
	"task-processor/internal/knowledge"
	knowledgehttp "task-processor/internal/knowledge/httpapi"
)

// WithKnowledge borrows a service prepared from the explicit Knowledge owner
// pool; its processor and database lifecycle belong to current-application.
func WithKnowledge(service *knowledge.Service) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.knowledgeServices++; o.knowledge = service }
}
func validateKnowledgeDescriptor(d httproute.Descriptor) error {
	for _, expected := range knowledgehttp.Routes(&knowledgehttp.Handler{}) {
		if d.Method == expected.Method && d.Path == expected.Path && d.Module == expected.Module && d.AuthPolicy == expected.AuthPolicy && d.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && d.OrganizationTargetResolver == nil && d.Permission == expected.Permission && d.RequestTimeout == expected.RequestTimeout && d.RejectUnreadRequestBody == expected.RejectUnreadRequestBody && d.Handler != nil {
			return nil
		}
	}
	return errors.New("knowledge route loses fresh content admission or bounded request contract")
}
