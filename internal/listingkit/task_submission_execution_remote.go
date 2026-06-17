package listingkit

import (
	"encoding/json"
	"strings"

	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

func (s *taskSubmissionExecutionService) executeSheinSubmitRemote(productAPI sheinproduct.ProductAPI, action string, submitProduct *sheinproduct.Product) (*sheinpub.SubmissionResponse, error) {
	switch action {
	case "save_draft":
		raw, _, err := productAPI.SaveDraftProduct(submitProduct)
		logSheinSubmitRemoteResponse(action, submitProduct, raw, err)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	case "publish":
		raw, _, err := productAPI.PublishProduct(submitProduct)
		logSheinSubmitRemoteResponse(action, submitProduct, raw, err)
		return sheinpub.BuildSubmissionResponseSummary(raw), err
	default:
		return nil, listingsubmission.UnsupportedSubmitActionError(action)
	}
}

func logSheinSubmitRemoteResponse(action string, submitProduct *sheinproduct.Product, raw *sheinproduct.SheinResponse, err error) {
	fields := logrus.Fields{
		"action":        action,
		"supplier_code": "",
		"spu_name":      "",
	}
	if submitProduct != nil {
		fields["supplier_code"] = strings.TrimSpace(submitProduct.SupplierCode)
		fields["spu_name"] = strings.TrimSpace(submitProduct.SPUName)
	}
	if raw != nil {
		fields["response_code"] = strings.TrimSpace(raw.Code)
		fields["response_msg"] = strings.TrimSpace(raw.Msg)
		if encoded, marshalErr := json.Marshal(raw); marshalErr == nil {
			fields["response_json"] = string(encoded)
		} else {
			fields["response_json_error"] = marshalErr.Error()
		}
	}
	if err != nil {
		fields["error"] = err.Error()
		logrus.WithFields(fields).Warn("listingkit shein submit remote completed with error")
		return
	}
	logrus.WithFields(fields).Info("listingkit shein submit remote response")
}
