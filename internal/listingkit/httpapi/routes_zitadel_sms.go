package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
)

type ZitadelSMSRouteHandler interface {
	DeliverZitadelSMS(*gin.Context)
}

func appendZitadelSMSRouteDescriptors(routes []httproute.Descriptor, handler ZitadelSMSRouteHandler) []httproute.Descriptor {
	return append(routes, httproute.Descriptor{
		Method:  http.MethodPost,
		Path:    "/api/v1/listing-kits/integrations/zitadel/sms",
		Module:  "listing-kit-zitadel-sms-webhook",
		Handler: handler.DeliverZitadelSMS,
	})
}
