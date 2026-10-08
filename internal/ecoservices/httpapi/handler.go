package httpapi

import (
	"context"
	"net/http"
	e "task-processor/internal/ecoservices"
)

type ServicePort interface {
	Read(context.Context, e.Query) (e.Page, error)
	Mutate(context.Context, e.Command) (e.Result, error)
}
type FilePort interface {
	Upload(context.Context, e.Scope, string, string, string, string, []byte) (e.File, error)
	DownloadForKind(context.Context, e.Scope, string, string) (e.File, []byte, error)
}
type PaymentPort interface {
	Checkout(context.Context, string, string, string) (string, error)
}
type Handler struct {
	service       ServicePort
	files         FilePort
	payments      PaymentPort
	notifications NotificationPort
	merchants     MerchantPort
	financial     FinancialPort
}
type MerchantPort interface {
	Resume(context.Context, e.Scope, string, int64) (e.MerchantView, error)
	Submit(context.Context, e.Scope, string, string, int64, e.MerchantDetails) (e.MerchantView, error)
	Read(context.Context, e.Scope, string) (e.MerchantView, error)
}

func (h *Handler) SetMerchants(p MerchantPort) { h.merchants = p }

type NotificationPort interface{ AcceptNotification(*http.Request) error }

func (h *Handler) SetNotifications(p NotificationPort) { h.notifications = p }

func NewHandler(service ServicePort, files FilePort, payments PaymentPort) (*Handler, error) {
	if service == nil || files == nil || payments == nil {
		return nil, e.ErrUnavailable
	}
	return &Handler{service: service, files: files, payments: payments}, nil
}
